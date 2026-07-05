// Package tracks manages a per-artist CSV catalog of track status - a
// local replacement for a spreadsheet tracking which projects are in
// progress, at what stage, and by whom. The header row is the schema:
// "Track" is the only required column (row identity), every other column
// is an opaque string and new ones can appear at any time without a code
// change.
package tracks

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// TrackColumn is the required identity column. Rows are matched by an
// exact, case-sensitive value in this column.
const TrackColumn = "Track"

// CatalogPath returns the per-artist track catalog path.
func CatalogPath(artistRoot string) string {
	return filepath.Join(artistRoot, ".abletonctl-tracks.csv")
}

// Row is one track's fields, keyed by column name. A missing key means the
// cell was empty.
type Row map[string]string

// Catalog is a per-artist track catalog: an ordered header plus rows.
type Catalog struct {
	Header []string
	Rows   []Row
}

// New returns an empty catalog with just the Track identity column, for
// starting a catalog from scratch.
func New() *Catalog {
	return &Catalog{Header: []string{TrackColumn}}
}

// Load reads a catalog from path. Rows with no value in TrackColumn (e.g.
// blank rows used as visual dividers in a spreadsheet) are skipped - they
// carry no data and are not preserved on Save.
func Load(path string) (*Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows from hand/spreadsheet edits
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty (expected a header row)", path)
	}

	header := records[0]
	if !contains(header, TrackColumn) {
		return nil, fmt.Errorf("%s has no %q column", path, TrackColumn)
	}

	cat := &Catalog{Header: header}
	for _, rec := range records[1:] {
		row := Row{}
		for i, col := range header {
			if i < len(rec) && rec[i] != "" {
				row[col] = rec[i]
			}
		}
		if row[TrackColumn] == "" {
			continue
		}
		cat.Rows = append(cat.Rows, row)
	}
	return cat, nil
}

// Save writes the catalog to path, one column per header entry in order.
func Save(path string, cat *Catalog) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(cat.Header); err != nil {
		return err
	}
	for _, row := range cat.Rows {
		rec := make([]string, len(cat.Header))
		for i, col := range cat.Header {
			rec[i] = row[col]
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// Find returns the index of the row whose Track column exactly matches
// name, or -1 if there is no such row.
func (c *Catalog) Find(name string) int {
	for i, row := range c.Rows {
		if row[TrackColumn] == name {
			return i
		}
	}
	return -1
}

// Add appends a new row for name, erroring if one already exists. Any key
// in fields not already in the header is appended to it.
func (c *Catalog) Add(name string, fields map[string]string) error {
	if c.Find(name) != -1 {
		return fmt.Errorf("track %q already exists (use 'track set' to update it)", name)
	}
	row := Row{TrackColumn: name}
	for k, v := range fields {
		c.ensureColumn(k)
		row[k] = v
	}
	c.Rows = append(c.Rows, row)
	return nil
}

// Set updates an existing row for name, erroring if none exists. Any key
// in fields not already in the header is appended to it.
func (c *Catalog) Set(name string, fields map[string]string) error {
	i := c.Find(name)
	if i == -1 {
		return fmt.Errorf("track %q not found (use 'track add' to create it)", name)
	}
	for k, v := range fields {
		c.ensureColumn(k)
		c.Rows[i][k] = v
	}
	return nil
}

func (c *Catalog) ensureColumn(name string) {
	if !contains(c.Header, name) {
		c.Header = append(c.Header, name)
	}
}

// DistinctValues returns the sorted set of non-empty values seen in column
// across all rows - a soft reference for freeform columns like Status,
// showing what vocabulary is already in use without enforcing it.
func (c *Catalog) DistinctValues(column string) []string {
	seen := map[string]bool{}
	for _, row := range c.Rows {
		if v := row[column]; v != "" {
			seen[v] = true
		}
	}
	values := make([]string, 0, len(seen))
	for v := range seen {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
