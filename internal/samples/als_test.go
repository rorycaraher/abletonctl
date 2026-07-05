package samples

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

const fixtureAls = `<?xml version="1.0" encoding="UTF-8"?>
<Ableton>
  <LiveSet>
    <Tracks>
      <AudioTrack>
        <DeviceChain>
          <Sample>
            <SampleRef>
              <FileRef>
                <RelativePathType Value="1" />
                <RelativePath Value="Samples\Imported\kick.wav" />
                <Path Value="/Users/rca/Music/artist-name/PRODUCTION-2026/Song/Samples/Imported/kick.wav" />
              </FileRef>
            </SampleRef>
          </Sample>
          <Sample>
            <SampleRef>
              <FileRef>
                <RelativePathType Value="1" />
                <RelativePath Value="Samples/Imported/snare.wav" />
                <Path Value="/Users/rca/Music/artist-name/PRODUCTION-2026/Song/Samples/Imported/snare.wav" />
              </FileRef>
            </SampleRef>
          </Sample>
        </DeviceChain>
      </AudioTrack>
    </Tracks>
  </LiveSet>
</Ableton>`

func writeFixtureAls(t *testing.T, path string) {
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
	if _, err := gz.Write([]byte(fixtureAls)); err != nil {
		t.Fatal(err)
	}
}

func TestExtractSampleRefs(t *testing.T) {
	dir := t.TempDir()
	alsPath := filepath.Join(dir, "Song.als")
	writeFixtureAls(t, alsPath)

	refs, err := ExtractSampleRefs(alsPath)
	if err != nil {
		t.Fatal(err)
	}

	if !refs.RelPaths["Samples/Imported/kick.wav"] {
		t.Errorf("expected backslash-separated RelativePath to be normalized to forward slashes")
	}
	if !refs.RelPaths["Samples/Imported/snare.wav"] {
		t.Errorf("expected snare.wav relative path to be captured")
	}
	if !refs.Filenames["kick.wav"] || !refs.Filenames["snare.wav"] {
		t.Errorf("expected filenames set to include both samples, got %v", refs.Filenames)
	}
}
