// Package samples parses Ableton .als files to find sample references and
// cross-checks them against files in a project's Samples folder.
package samples

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// SampleRefs is the set of sample references found across one or more .als
// files, split by how strong a match they provide.
type SampleRefs struct {
	// RelPaths are project-relative paths (e.g. "Samples/Imported/foo.wav")
	// taken from <RelativePath Value="..."/> elements. These are the
	// reliable signal: Ableton keeps them consistent within a project
	// regardless of which machine/user last opened it.
	RelPaths map[string]bool
	// Filenames is every basename seen in any RelativePath or absolute
	// Path element. Used only as a weaker fallback signal when a file
	// doesn't match by relative path.
	Filenames map[string]bool
}

func newSampleRefs() *SampleRefs {
	return &SampleRefs{RelPaths: map[string]bool{}, Filenames: map[string]bool{}}
}

// ExtractSampleRefs scans the gzipped XML of an .als file for sample
// references. It scans generically for RelativePath/Path elements rather
// than unmarshaling a fixed schema, since Ableton's XML shape has drifted
// across Live versions.
func ExtractSampleRefs(alsPath string) (*SampleRefs, error) {
	f, err := os.Open(alsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: not a gzipped .als file: %w", alsPath, err)
	}
	defer gz.Close()

	return scanSampleRefs(gz)
}

func scanSampleRefs(r io.Reader) (*SampleRefs, error) {
	refs := newSampleRefs()
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing als xml: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "RelativePath":
			if v, ok := attrValue(se); ok && v != "" {
				norm := normalizeRelPath(v)
				refs.RelPaths[norm] = true
				refs.Filenames[path.Base(norm)] = true
			}
		case "Path":
			if v, ok := attrValue(se); ok && v != "" {
				refs.Filenames[path.Base(normalizeRelPath(v))] = true
			}
		}
	}
	return refs, nil
}

func attrValue(se xml.StartElement) (string, bool) {
	for _, a := range se.Attr {
		if a.Name.Local == "Value" {
			return a.Value, true
		}
	}
	return "", false
}

func normalizeRelPath(p string) string {
	return strings.ReplaceAll(strings.TrimSpace(p), `\`, "/")
}

// Merge folds other into refs.
func (refs *SampleRefs) Merge(other *SampleRefs) {
	for k := range other.RelPaths {
		refs.RelPaths[k] = true
	}
	for k := range other.Filenames {
		refs.Filenames[k] = true
	}
}
