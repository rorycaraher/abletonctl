package tracks

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_SkipsBlankDividerRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abletonctl-tracks.csv")
	writeFile(t, path, "Track,Status,Allocation\nZap Dub,Development,Float\n,,\nKickphase,Development,Float\n")

	cat, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(cat.Rows), cat.Rows)
	}
	if cat.Rows[0][TrackColumn] != "Zap Dub" || cat.Rows[1][TrackColumn] != "Kickphase" {
		t.Fatalf("unexpected rows: %+v", cat.Rows)
	}
}

func TestLoad_MissingTrackColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abletonctl-tracks.csv")
	writeFile(t, path, "Status,Allocation\nDevelopment,Float\n")

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing Track column")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abletonctl-tracks.csv")

	_, err := Load(path)
	if !os.IsNotExist(err) {
		t.Fatalf("got %v, want a not-exist error", err)
	}
}

func TestAddThenSet_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abletonctl-tracks.csv")

	cat := New()
	if err := cat.Add("Zap Dub", map[string]string{"Status": "Idea"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, cat); err != nil {
		t.Fatal(err)
	}

	cat, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Set("Zap Dub", map[string]string{"Status": "Development", "Priority": "B"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, cat); err != nil {
		t.Fatal(err)
	}

	cat, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(cat.Rows))
	}
	row := cat.Rows[0]
	if row["Status"] != "Development" || row["Priority"] != "B" {
		t.Fatalf("unexpected row after set: %+v", row)
	}
	if !contains(cat.Header, "Priority") {
		t.Fatalf("expected Priority to be added to header: %v", cat.Header)
	}
}

func TestAdd_RejectsDuplicate(t *testing.T) {
	cat := New()
	if err := cat.Add("Zap Dub", nil); err != nil {
		t.Fatal(err)
	}
	if err := cat.Add("Zap Dub", nil); err == nil {
		t.Fatal("expected error adding a duplicate track")
	}
}

func TestSet_RejectsMissing(t *testing.T) {
	cat := New()
	if err := cat.Set("Zap Dub", map[string]string{"Status": "Idea"}); err == nil {
		t.Fatal("expected error setting a nonexistent track")
	}
}

func TestDistinctValues(t *testing.T) {
	cat := New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(cat.Add("A", map[string]string{"Status": "Idea"}))
	must(cat.Add("B", map[string]string{"Status": "Development"}))
	must(cat.Add("C", map[string]string{"Status": "Idea"}))
	must(cat.Add("D", nil))

	got := cat.DistinctValues("Status")
	want := []string{"Development", "Idea"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSave_PreservesColumnOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abletonctl-tracks.csv")
	writeFile(t, path, "Track,Status,Priority\nZap Dub,Development,B\n")

	cat, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Set("Zap Dub", map[string]string{"Studio": "Home"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, cat); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "Track,Status,Priority,Studio\nZap Dub,Development,B,Home\n"
	if string(raw) != want {
		t.Fatalf("got %q, want %q", string(raw), want)
	}
}
