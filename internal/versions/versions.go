// Package versions picks the latest Version of a Project by the version
// number in its file name, never by modification time (a re-saved old
// Version would otherwise look newest).
package versions

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rorycaraher/abletonctl/internal/demolink"
)

// Latest returns the .als with the highest version number among alsFiles.
//
// A file's version key is its name without .als, cut at the first `__`
// (so `bell-04__alt.als` has key `bell-04`). Keys are compared naturally:
// split into digit and non-digit runs, digit runs compared by value. That
// orders `bell-04` < `bell-04-2026` < `bell-04-2026-02`, which comparing
// only the trailing number would get wrong (2026 > 02). Files whose key has
// no digits are not Versions in this sense and are ignored.
//
// If the top key is shared by several files (`song-03.als` and
// `song-03__alt.als`) the answer is not guessed: an error lists them.
func Latest(alsFiles []string) (string, error) {
	type cand struct{ path, key string }
	var cands []cand
	for _, p := range alsFiles {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		key, _, _ := demolink.Parse(stem)
		if !hasDigit(key) {
			continue
		}
		cands = append(cands, cand{p, key})
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no .als file with a version number in its name")
	}
	sort.SliceStable(cands, func(i, j int) bool { return naturalLess(cands[i].key, cands[j].key) })

	top := cands[len(cands)-1]
	var tied []string
	for _, c := range cands {
		if naturalCompare(c.key, top.key) == 0 {
			tied = append(tied, filepath.Base(c.path))
		}
	}
	if len(tied) > 1 {
		return "", fmt.Errorf("latest version %q is ambiguous: %s", top.key, strings.Join(tied, ", "))
	}
	return top.path, nil
}

func hasDigit(s string) bool {
	return strings.ContainsAny(s, "0123456789")
}

func naturalLess(a, b string) bool { return naturalCompare(a, b) < 0 }

// naturalCompare returns -1, 0 or 1. Digit runs compare by numeric value;
// equal-valued runs written differently (`04` vs `4`) compare as equal.
func naturalCompare(a, b string) int {
	ta, tb := tokens(a), tokens(b)
	for i := 0; i < len(ta) && i < len(tb); i++ {
		x, y := ta[i], tb[i]
		xd, yd := isDigit(x[0]), isDigit(y[0])
		if xd && yd {
			x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
			if len(x) != len(y) {
				return cmpInt(len(x), len(y))
			}
		}
		if c := strings.Compare(x, y); c != 0 {
			return c
		}
	}
	return cmpInt(len(ta), len(tb))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// tokens splits s into alternating runs of digits and non-digits.
func tokens(s string) []string {
	var out []string
	start := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || isDigit(s[i]) != isDigit(s[i-1]) {
			out = append(out, s[start:i])
			start = i
		}
	}
	return out
}
