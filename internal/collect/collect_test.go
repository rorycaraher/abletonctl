package collect

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildAls writes a gzipped .als at path from raw XML content.
func buildAls(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

func readAls(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := gz.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}

func sampleRefXML(path string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<Ableton>
  <LiveSet>
    <Tracks>
      <AudioTrack>
        <DeviceChain>
          <Sample>
            <SampleRef>
              <FileRef>
                <RelativePathType Value="1" />
                <RelativePath Value="kick.wav" />
                <Path Value="` + path + `" />
                <LivePackId Value="" />
              </FileRef>
              <LastModDate Value="1000" />
            </SampleRef>
          </Sample>
        </DeviceChain>
      </AudioTrack>
    </Tracks>
  </LiveSet>
</Ableton>`
}

func TestCollectOne_CollectsExternalSample(t *testing.T) {
	dir := t.TempDir()
	externalDir := t.TempDir()

	externalWav := filepath.Join(externalDir, "kick.wav")
	if err := os.WriteFile(externalWav, []byte("fake audio data"), 0o644); err != nil {
		t.Fatal(err)
	}

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, sampleRefXML(externalWav))

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Collected {
		t.Fatalf("expected Collected, got %v (problems: %v)", result.Status, result.Problems)
	}
	if result.Count != 1 || result.Unique != 1 {
		t.Errorf("expected count=1 unique=1, got count=%d unique=%d", result.Count, result.Unique)
	}

	destPath := filepath.Join(dir, "Samples", "Imported", "kick.wav")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("expected collected file at %s: %v", destPath, err)
	}

	// Original untouched.
	if _, err := os.Stat(alsPath); err != nil {
		t.Errorf("original .als should be untouched: %v", err)
	}

	out := readAls(t, result.Output)
	if !strings.Contains(out, `Path Value="`+destPath+`"`) {
		t.Errorf("expected patched Path to point at %s, got:\n%s", destPath, out)
	}
	if !strings.Contains(out, `RelativePath Value="Samples/Imported/kick.wav"`) {
		t.Errorf("expected patched RelativePath, got:\n%s", out)
	}
	if !strings.Contains(out, `RelativePathType Value="3"`) {
		t.Errorf("expected RelativePathType rewritten to 3, got:\n%s", out)
	}
}

func TestCollectOne_SkipsPackContent(t *testing.T) {
	dir := t.TempDir()
	externalDir := t.TempDir()
	externalWav := filepath.Join(externalDir, "kick.wav")
	os.WriteFile(externalWav, []byte("data"), 0o644)

	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Ableton><LiveSet><Tracks><AudioTrack><DeviceChain><Sample><SampleRef>
  <FileRef>
    <RelativePathType Value="1" />
    <RelativePath Value="kick.wav" />
    <Path Value="` + externalWav + `" />
    <LivePackId Value="some-pack-id" />
  </FileRef>
  <LastModDate Value="1000" />
</SampleRef></Sample></DeviceChain></AudioTrack></Tracks></LiveSet></Ableton>`

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, xml)

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Nothing {
		t.Fatalf("expected Nothing (Pack content skipped), got %v", result.Status)
	}
}

func TestCollectOne_SkipsBuiltinContent(t *testing.T) {
	dir := t.TempDir()
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Ableton><LiveSet><Tracks><AudioTrack><DeviceChain><Sample><SampleRef>
  <FileRef>
    <RelativePathType Value="1" />
    <RelativePath Value="kick.wav" />
    <Path Value="/Applications/Ableton Live 12 Suite.app/Contents/App-Resources/Core Library/kick.wav" />
    <LivePackId Value="" />
  </FileRef>
  <LastModDate Value="1000" />
</SampleRef></Sample></DeviceChain></AudioTrack></Tracks></LiveSet></Ableton>`

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, xml)

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Nothing {
		t.Fatalf("expected Nothing (Builtin content skipped), got %v", result.Status)
	}
}

func TestCollectOne_SkipsAlreadyLocal(t *testing.T) {
	dir := t.TempDir()
	localWav := filepath.Join(dir, "Samples", "Imported", "kick.wav")
	os.MkdirAll(filepath.Dir(localWav), 0o755)
	os.WriteFile(localWav, []byte("data"), 0o644)

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, sampleRefXML(localWav))

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Nothing {
		t.Fatalf("expected Nothing (already local), got %v", result.Status)
	}
}

func TestCollectOne_MissingSourceFails(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone.wav")

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, sampleRefXML(missing))

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Failed {
		t.Fatalf("expected Failed (missing source), got %v", result.Status)
	}

	// Nothing should have been written.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected only the original .als in %s, got %v", dir, entries)
	}
}

func TestCollectOne_DestinationCollisionFails(t *testing.T) {
	dir := t.TempDir()

	existing := filepath.Join(dir, "Samples", "Imported", "kick.wav")
	os.MkdirAll(filepath.Dir(existing), 0o755)
	os.WriteFile(existing, []byte("existing content"), 0o644)

	externalWav := filepath.Join(t.TempDir(), "kick.wav")
	os.WriteFile(externalWav, []byte("different content"), 0o644)

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, sampleRefXML(externalWav))

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Failed {
		t.Fatalf("expected Failed (destination collision), got %v", result.Status)
	}

	// The existing file must be untouched.
	data, _ := os.ReadFile(existing)
	if string(data) != "existing content" {
		t.Errorf("existing destination file was modified: %q", data)
	}
}

func TestCollectOne_ReusesIdenticalExistingDestination(t *testing.T) {
	dir := t.TempDir()

	content := []byte("same bytes")
	existing := filepath.Join(dir, "Samples", "Imported", "kick.wav")
	os.MkdirAll(filepath.Dir(existing), 0o755)
	os.WriteFile(existing, content, 0o644)

	externalWav := filepath.Join(t.TempDir(), "kick.wav")
	os.WriteFile(externalWav, content, 0o644)

	alsPath := filepath.Join(dir, "Song.als")
	buildAls(t, alsPath, sampleRefXML(externalWav))

	result, err := CollectOne(alsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != Collected {
		t.Fatalf("expected Collected, got %v (problems: %v)", result.Status, result.Problems)
	}
	if len(result.Report) != 1 || !strings.HasPrefix(result.Report[0], "reused") {
		t.Errorf("expected a single 'reused' report line, got %v", result.Report)
	}
}

func TestNextOutputPath(t *testing.T) {
	dir := t.TempDir()

	p, err := NextOutputPath(filepath.Join(dir, "Song.als"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "Song-01.als" {
		t.Errorf("expected Song-01.als, got %s", filepath.Base(p))
	}

	os.WriteFile(filepath.Join(dir, "Song-01.als"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "Song-02.als"), nil, 0o644)

	p, err = NextOutputPath(filepath.Join(dir, "Song-01.als"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "Song-03.als" {
		t.Errorf("expected Song-03.als (skipping taken -02), got %s", filepath.Base(p))
	}
}
