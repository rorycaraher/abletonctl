// Package collect ports Ableton's "Collect All and Save" (File menu) into a
// scriptable operation on a .als file.
//
// Reverse-engineered from real before/after .als diffs (Live 12.4.2), not
// Ableton's docs -- deliberately narrower than the real feature:
//
// Collects (flattened, not mirroring Ableton's own subfolder-preservation
// heuristic):
//   - audio       <SampleRef>/<FileRef>  -> Samples/Imported/<basename>
//   - M4L devices <MxPatchRef>/<FileRef> -> Presets/Imported/<basename>
//
// (identical FileRef shape, same rules for both)
//
// Leaves alone: Pack content (LivePackId set), Ableton's own bundled
// "Builtin" content (path contains ".app/Contents/"), anything already
// inside the project, and everything else -- racks, presets, VST state.
package collect

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// containerDestRoots maps a FileRef container element to the top-level
// project folder its collected content lands in (under an "Imported"
// subfolder).
var containerDestRoots = map[string]string{
	"SampleRef":  "Samples",
	"MxPatchRef": "Presets",
}

// trackedFileRefFields are the direct children of a FileRef this package
// reads. RelativePathType/RelativePath/Path get rewritten to point at the
// collected copy; LivePackId (if present) marks Pack content, which is
// always left alone.
var trackedFileRefFields = map[string]bool{
	"RelativePathType": true,
	"RelativePath":     true,
	"Path":             true,
	"LivePackId":       true,
}

// fieldLoc is a captured attribute value plus the 1-based line it lives on,
// so it can be patched in place later without re-serializing the XML.
type fieldLoc struct {
	line  int
	value string
}

// fileRefContext is one SampleRef/MxPatchRef element's tracked fields,
// collected during the scan.
type fileRefContext struct {
	container    string
	fields       map[string]fieldLoc
	fileRefDepth int // -1 until a direct-child FileRef is seen
}

// Status is the outcome of a single-file collect run.
type Status int

const (
	// Failed means nothing was copied or written; see Result.Problems.
	Failed Status = iota
	// Nothing means no external content was found to collect.
	Nothing
	// Collected means files were copied and a new .als was written.
	Collected
)

// Result is the outcome of CollectOne.
type Result struct {
	Status Status
	// Problems explains a Failed result: one entry per issue found.
	Problems []string
	// Report is a human-readable line per file copied or reused, only set
	// on a Collected result.
	Report []string
	// Count is the number of FileRef entries rewritten; Unique is the
	// number of distinct files copied (several entries can share one
	// destination). Only set on a Collected result.
	Count, Unique int
	// DestDirs are the distinct destination directories written into.
	DestDirs []string
	// Output is the path of the newly written .als file.
	Output string
}

// CollectOne runs Collect All and Save for one .als file. It never
// overwrites alsPath; on success it writes a new numbered file alongside it.
// A returned error means the input couldn't even be read/parsed; anything
// that fails validation once parsing succeeds is reported via a Failed
// Result instead.
func CollectOne(alsPath string) (Result, error) {
	alsPath, err := filepath.Abs(alsPath)
	if err != nil {
		return Result{}, err
	}
	projectRoot := filepath.Dir(alsPath)

	raw, err := readGzip(alsPath)
	if err != nil {
		return Result{}, err
	}
	lines := splitLines(string(raw))

	contexts, err := scanFileRefs(bytes.NewReader(raw))
	if err != nil {
		return Result{}, err
	}

	toCollect, problems := analyze(contexts, projectRoot, lines)
	if len(problems) > 0 {
		return Result{Status: Failed, Problems: problems}, nil
	}
	if len(toCollect) == 0 {
		return Result{Status: Nothing}, nil
	}

	return applyCollect(alsPath, projectRoot, toCollect, lines)
}

func readGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: not a gzipped .als file: %w", path, err)
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// scanFileRefs walks the document tracking line numbers, recording -- for
// each SampleRef/MxPatchRef -- the direct-child FileRef's tracked fields.
// Nested FileRefs (e.g. under SourceContext/OriginalFileRef) are ignored
// because they never sit at the tracked FileRef's exact depth with parent
// equal to the container element.
func scanFileRefs(r io.Reader) ([]*fileRefContext, error) {
	dec := xml.NewDecoder(r)

	var stack []string
	var contextStack []*fileRefContext
	var results []*fileRefContext

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing als xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			parentDepth := len(stack)
			var parentName string
			if parentDepth > 0 {
				parentName = stack[parentDepth-1]
			}

			if _, ok := containerDestRoots[name]; ok {
				contextStack = append(contextStack, &fileRefContext{
					container:    name,
					fields:       map[string]fieldLoc{},
					fileRefDepth: -1,
				})
			} else if len(contextStack) > 0 {
				current := contextStack[len(contextStack)-1]
				line, _ := dec.InputPos()
				switch {
				case name == "FileRef" && parentName == current.container && current.fileRefDepth == -1:
					current.fileRefDepth = parentDepth + 1
				case name == "LastModDate" && parentName == current.container:
					current.fields["LastModDate"] = fieldLoc{line: line, value: attrValue(t)}
				case current.fileRefDepth != -1 && parentDepth == current.fileRefDepth &&
					parentName == "FileRef" && trackedFileRefFields[name]:
					current.fields[name] = fieldLoc{line: line, value: attrValue(t)}
				}
			}

			stack = append(stack, name)

		case xml.EndElement:
			stack = stack[:len(stack)-1]
			if _, ok := containerDestRoots[t.Name.Local]; ok && len(contextStack) > 0 {
				n := len(contextStack) - 1
				results = append(results, contextStack[n])
				contextStack = contextStack[:n]
			}
		}
	}
	return results, nil
}

func attrValue(se xml.StartElement) string {
	for _, a := range se.Attr {
		if a.Name.Local == "Value" {
			return a.Value
		}
	}
	return ""
}

// collectEntry is one FileRef this run will rewrite: its external source,
// where it lands, and which lines need patching.
type collectEntry struct {
	source                                              string
	dest                                                string
	relPathTypeLine, relPathLine, pathLine, lastModLine int
}

// analyze decides what to collect and what's wrong, touching nothing on
// disk except reads for hashing/existence checks. Every line that would
// need patching is validated against the expected one-tag-per-line format
// here, before the caller copies or writes anything.
func analyze(contexts []*fileRefContext, projectRoot string, lines []string) ([]collectEntry, []string) {
	var toCollect []collectEntry
	var problems []string

	required := []string{"RelativePathType", "RelativePath", "Path", "LastModDate"}

	for _, ctx := range contexts {
		if ctx.fileRefDepth == -1 {
			continue // no FileRef under this container at all
		}

		if lp, ok := ctx.fields["LivePackId"]; ok && lp.value != "" {
			continue // Pack content, always skip
		}

		pathField, ok := ctx.fields["Path"]
		if !ok || pathField.value == "" {
			continue
		}
		sourcePath := pathField.value

		if strings.Contains(sourcePath, ".app/Contents/") {
			continue // Ableton "Builtin" content bundled with the app itself
		}

		if isWithin(projectRoot, sourcePath) {
			continue // already local
		}

		var missing []string
		for _, f := range required {
			if _, ok := ctx.fields[f]; !ok {
				missing = append(missing, f)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf(
				"unrecognized %s structure for %s (missing field(s): %s) -- refusing to guess",
				ctx.container, sourcePath, strings.Join(missing, ", ")))
			continue
		}

		if fi, err := os.Stat(sourcePath); err != nil || fi.IsDir() {
			problems = append(problems, fmt.Sprintf("source file missing on disk: %s", sourcePath))
			continue
		}

		var unpatchable []string
		for _, f := range required {
			if !linePatchable(lines[ctx.fields[f].line-1]) {
				unpatchable = append(unpatchable, f)
			}
		}
		if len(unpatchable) > 0 {
			problems = append(problems, fmt.Sprintf(
				"unrecognized line format for %s (field(s): %s) -- refusing to guess",
				sourcePath, strings.Join(unpatchable, ", ")))
			continue
		}

		destRoot := containerDestRoots[ctx.container]
		dest := filepath.Join(projectRoot, destRoot, "Imported", filepath.Base(sourcePath))

		toCollect = append(toCollect, collectEntry{
			source:          sourcePath,
			dest:            dest,
			relPathTypeLine: ctx.fields["RelativePathType"].line,
			relPathLine:     ctx.fields["RelativePath"].line,
			pathLine:        ctx.fields["Path"].line,
			lastModLine:     ctx.fields["LastModDate"].line,
		})
	}

	problems = append(problems, checkDestCollisions(toCollect)...)
	return toCollect, problems
}

// isWithin reports whether target is root itself or a descendant of it,
// comparing paths syntactically (no symlink resolution), matching Ableton's
// own Path attributes which are plain strings.
func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

// checkDestCollisions flags any destination that would receive content from
// sources with different contents, or that already exists locally with
// different content than what would be copied there.
func checkDestCollisions(toCollect []collectEntry) []string {
	destToSources := map[string]map[string]bool{}
	for _, e := range toCollect {
		if destToSources[e.dest] == nil {
			destToSources[e.dest] = map[string]bool{}
		}
		destToSources[e.dest][e.source] = true
	}

	dests := make([]string, 0, len(destToSources))
	for d := range destToSources {
		dests = append(dests, d)
	}
	sort.Strings(dests)

	hashCache := map[string]string{}
	var problems []string

	for _, dest := range dests {
		sources := make([]string, 0, len(destToSources[dest]))
		for s := range destToSources[dest] {
			sources = append(sources, s)
		}
		sort.Strings(sources)

		type named struct {
			path string
			hash string
		}
		var entries []named
		distinct := map[string]bool{}
		for _, s := range sources {
			h, err := sha256Of(s, hashCache)
			if err != nil {
				problems = append(problems, fmt.Sprintf("hashing %s: %v", s, err))
				continue
			}
			entries = append(entries, named{s, h})
			distinct[h] = true
		}
		if _, err := os.Stat(dest); err == nil {
			if h, err := sha256Of(dest, hashCache); err == nil {
				entries = append(entries, named{dest, h})
				distinct[h] = true
			}
		}

		if len(distinct) > 1 {
			parts := make([]string, len(entries))
			for i, e := range entries {
				parts[i] = fmt.Sprintf("%s (%s)", e.path, e.hash[:8])
			}
			problems = append(problems, fmt.Sprintf("destination collision at %s: %s", dest, strings.Join(parts, ", ")))
		}
	}

	return problems
}

func sha256Of(path string, cache map[string]string) (string, error) {
	if h, ok := cache[path]; ok {
		return h, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	cache[path] = sum
	return sum, nil
}

// applyCollect copies each entry's source (once per unique destination),
// patches the four tracked lines per entry, and writes a new numbered .als.
// Any copy performed is tracked so it can be rolled back if a later step
// fails -- the goal being that a crash here still leaves either everything
// done or nothing done.
func applyCollect(alsPath, projectRoot string, toCollect []collectEntry, lines []string) (Result, error) {
	for _, destRoot := range containerDestRoots {
		if err := os.MkdirAll(filepath.Join(projectRoot, destRoot, "Imported"), 0o755); err != nil {
			return Result{}, err
		}
	}

	runTimestamp := strconv.FormatInt(time.Now().Unix(), 10)

	copied := map[string]bool{}
	var newlyCreated []string
	var report []string
	edits := map[int]string{}

	rollback := func() {
		for _, p := range newlyCreated {
			os.Remove(p)
		}
	}

	for _, entry := range toCollect {
		if !copied[entry.dest] {
			if _, err := os.Stat(entry.dest); os.IsNotExist(err) {
				if err := copyFile(entry.source, entry.dest); err != nil {
					rollback()
					return Result{Status: Failed, Problems: []string{
						fmt.Sprintf("unexpected error, rolled back %d copied file(s): %v", len(newlyCreated), err),
					}}, nil
				}
				newlyCreated = append(newlyCreated, entry.dest)
				report = append(report, fmt.Sprintf("copied  %s -> %s", entry.source, entry.dest))
			} else {
				report = append(report, fmt.Sprintf("reused  %s (already present, identical content)", entry.dest))
			}
			copied[entry.dest] = true
		}

		relPath, err := filepath.Rel(projectRoot, entry.dest)
		if err != nil {
			rollback()
			return Result{}, err
		}
		relPath = filepath.ToSlash(relPath)

		patches := map[int]string{
			entry.relPathTypeLine: "3",
			entry.relPathLine:     relPath,
			entry.pathLine:        entry.dest,
			entry.lastModLine:     runTimestamp,
		}
		for lineNo, value := range patches {
			patched, err := replaceAttrValue(lines[lineNo-1], value)
			if err != nil {
				rollback()
				return Result{Status: Failed, Problems: []string{
					fmt.Sprintf("unexpected error, rolled back %d copied file(s): %v", len(newlyCreated), err),
				}}, nil
			}
			edits[lineNo] = patched
		}
	}

	for lineNo, patched := range edits {
		lines[lineNo-1] = patched
	}

	outputPath, err := NextOutputPath(alsPath)
	if err != nil {
		rollback()
		return Result{}, err
	}
	if err := writeGzip(outputPath, strings.Join(lines, "")); err != nil {
		rollback()
		return Result{Status: Failed, Problems: []string{
			fmt.Sprintf("unexpected error, rolled back %d copied file(s): %v", len(newlyCreated), err),
		}}, nil
	}

	destDirSet := map[string]bool{}
	for dest := range copied {
		destDirSet[filepath.Dir(dest)] = true
	}
	destDirs := make([]string, 0, len(destDirSet))
	for d := range destDirSet {
		destDirs = append(destDirs, d)
	}
	sort.Strings(destDirs)

	return Result{
		Status:   Collected,
		Report:   report,
		Count:    len(toCollect),
		Unique:   len(copied),
		DestDirs: destDirs,
		Output:   outputPath,
	}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func writeGzip(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := gz.Write([]byte(content)); err != nil {
		gz.Close()
		return err
	}
	return gz.Close()
}

// NextOutputPath returns the next numbered filename, skipping past any that
// already exist -- e.g. if a directory already has -01 through -03, a -01
// input's output lands at -04, not -02. Never returns a path that already
// exists.
func NextOutputPath(inputPath string) (string, error) {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)

	if m := numberedNameRE.FindStringSubmatch(base); m != nil {
		prefix, numStr, suffix := m[1], m[2], m[3]
		width := len(numStr)
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return "", err
		}
		for {
			n++
			candidate := filepath.Join(dir, fmt.Sprintf("%s%0*d%s", prefix, width, n, suffix))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				return candidate, nil
			}
		}
	}

	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 1; ; n++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%02d%s", stem, n, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
}

var numberedNameRE = regexp.MustCompile(`^(.*-)(\d{2,})(\.als)$`)

var patchLineRE = regexp.MustCompile(`(<\w+\s+Value=")[^"]*("\s*/>\s*)$`)

func linePatchable(line string) bool {
	content, _ := stripLineEnding(line)
	return patchLineRE.MatchString(content)
}

// replaceAttrValue rewrites the Value="..." attribute on a single tracked
// line, preserving everything else on the line (tag name, closing syntax,
// original line ending) byte-for-byte.
func replaceAttrValue(line, newValue string) (string, error) {
	content, ending := stripLineEnding(line)
	escaped := escapeAttrValue(newValue)

	m := patchLineRE.FindStringSubmatchIndex(content)
	if m == nil {
		return "", fmt.Errorf("could not patch line, format not recognized: %q", line)
	}
	var b strings.Builder
	b.WriteString(content[:m[2]])
	b.WriteString(content[m[2]:m[3]])
	b.WriteString(escaped)
	b.WriteString(content[m[4]:m[5]])
	b.WriteString(content[m[5]:])
	return b.String() + ending, nil
}

var attrEscaper = strings.NewReplacer(
	"&", "&amp;",
	`"`, "&quot;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeAttrValue(v string) string {
	return attrEscaper.Replace(v)
}

func stripLineEnding(line string) (content, ending string) {
	if strings.HasSuffix(line, "\r\n") {
		return line[:len(line)-2], "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return line[:len(line)-1], "\n"
	}
	return line, ""
}

// splitLines splits s into lines, each retaining its own trailing "\n" or
// "\r\n" (the last line has none if s doesn't end in one), so the original
// bytes can be reconstructed exactly by concatenating unmodified lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
