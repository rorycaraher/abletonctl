package demos

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// minimalWav builds a valid 16-bit mono PCM WAV file of silence, small
// enough to encode instantly but real enough for ffmpeg to decode.
func minimalWav(numSamples int) []byte {
	const (
		channels   = 1
		sampleRate = 44100
		bitsPerSmp = 16
	)
	dataSize := numSamples * channels * (bitsPerSmp / 8)
	byteRate := sampleRate * channels * (bitsPerSmp / 8)
	blockAlign := channels * (bitsPerSmp / 8)

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSmp))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(make([]byte, dataSize))
	return buf.Bytes()
}

func requireFfmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH, skipping conversion test")
	}
}

func TestDiscoverSourceFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "track-one.aiff"), []byte{})
	mustWrite(t, filepath.Join(dir, "track-two.wav"), []byte{})
	mustWrite(t, filepath.Join(dir, "already-done.mp3"), []byte{})
	mustWrite(t, filepath.Join(dir, ".DS_Store"), []byte{})
	mustWrite(t, filepath.Join(dir, "subfolder", "track-three.aif"), []byte{})

	files, err := DiscoverSourceFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	var gotBase []string
	for _, f := range files {
		gotBase = append(gotBase, filepath.Base(f))
	}
	want := []string{"track-one.aiff", "track-two.wav", "track-three.aif"}
	sortStrings(gotBase)
	sortStrings(want)
	if !reflect.DeepEqual(gotBase, want) {
		t.Fatalf("got %v, want %v", gotBase, want)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func TestMp3Path(t *testing.T) {
	if got := Mp3Path("/a/b/Track One.aiff"); got != "/a/b/Track One.mp3" {
		t.Fatalf("got %q", got)
	}
	if got := Mp3Path("/a/b/Track Two.wav"); got != "/a/b/Track Two.mp3" {
		t.Fatalf("got %q", got)
	}
}

func TestConvertArgs(t *testing.T) {
	args := ConvertArgs("/in.wav", "/out.mp3")
	want := []string{
		"-hide_banner", "-loglevel", "warning",
		"-y",
		"-i", "/in.wav",
		"-codec:a", "libmp3lame", "-b:a", "320k",
		"/out.mp3",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %v, want %v", args, want)
	}
}

func TestConvertAndCleanup_SuccessDeletesOriginal(t *testing.T) {
	requireFfmpeg(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "demo.wav")
	mustWrite(t, src, minimalWav(4410))

	outcomes, err := ConvertAndCleanup(dir, false, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1: %+v", len(outcomes), outcomes)
	}
	o := outcomes[0]
	if o.Err != nil {
		t.Fatalf("unexpected error: %v", o.Err)
	}
	if !o.Deleted {
		t.Fatalf("expected original to be deleted")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("expected source to be gone, stat err = %v", err)
	}
	info, err := os.Stat(o.Mp3)
	if err != nil {
		t.Fatalf("expected mp3 to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("expected non-empty mp3")
	}
}

func TestConvertAndCleanup_FailureKeepsOriginal(t *testing.T) {
	requireFfmpeg(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "corrupt.wav")
	mustWrite(t, src, []byte("this is not a wav file"))

	outcomes, err := ConvertAndCleanup(dir, false, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Err == nil {
		t.Fatalf("expected a conversion error, got %+v", outcomes)
	}
	if outcomes[0].Deleted {
		t.Fatalf("must not delete original on conversion failure")
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("expected source to still exist: %v", err)
	}
}

func TestConvertAndCleanup_DryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "demo.aiff")
	mustWrite(t, src, minimalWav(100))

	outcomes, err := ConvertAndCleanup(dir, true, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Deleted {
		t.Fatalf("dry-run must not report anything as deleted: %+v", outcomes)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("dry-run must not touch source: %v", err)
	}
	if _, err := os.Stat(outcomes[0].Mp3); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create mp3 output")
	}
}
