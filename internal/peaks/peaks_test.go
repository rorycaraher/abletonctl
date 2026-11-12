package peaks

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const rate = 1000 // Hz: low, so Hold is a round 100 frames

// signal is frames of samples, one per channel, as a fraction of full scale.
type signal [][]float64

// testSignal is 3s of silence with, over -6 dB unless noted:
//   - a burst at 0.50-0.55s peaking at 0.95 on frame 520, and another at
//     0.56-0.58s, close enough to merge with it;
//   - a burst at 1.50-1.52s at 0.6, on the second channel only;
//   - a burst at 2.00-2.02s at 0.4 (-8 dB), under the default threshold.
func testSignal() signal {
	s := make(signal, 3*rate)
	for i := range s {
		s[i] = []float64{0, 0}
	}
	burst := func(from, to, ch int, amp float64) {
		for i := from; i < to; i++ {
			v := amp
			if i%2 == 1 {
				v = -amp // a crude waveform: it crosses back under on every cycle
			}
			s[i][ch] = v
		}
	}
	burst(500, 550, 0, 0.9)
	s[520][0] = 0.95
	burst(560, 580, 0, 0.9)
	burst(1500, 1520, 1, 0.6)
	burst(2000, 2020, 0, 0.4)
	return s
}

var wantDefault = []Peak{
	{Time: 520 * time.Millisecond, Level: 20 * math.Log10(0.95)},
	{Time: 1500 * time.Millisecond, Level: 20 * math.Log10(0.6)},
}

type enc struct {
	name  string
	build func(signal) []byte
	tol   float64 // dB: quantisation error allowed in reported levels
}

func encodings() []enc {
	return []enc{
		{"wav 16-bit", func(s signal) []byte { return wav(s, wavPCM, 16, false) }, 0.01},
		{"wav 24-bit", func(s signal) []byte { return wav(s, wavPCM, 24, false) }, 0.001},
		{"wav 32-bit", func(s signal) []byte { return wav(s, wavPCM, 32, false) }, 0.001},
		{"wav 8-bit", func(s signal) []byte { return wav(s, wavPCM, 8, false) }, 0.2},
		{"wav float32", func(s signal) []byte { return wav(s, wavFloat, 32, false) }, 0.001},
		{"wav float64", func(s signal) []byte { return wav(s, wavFloat, 64, false) }, 0.001},
		{"wav extensible 24-bit", func(s signal) []byte { return wav(s, wavPCM, 24, true) }, 0.001},
		{"wav extensible float32", func(s signal) []byte { return wav(s, wavFloat, 32, true) }, 0.001},
		{"aiff 16-bit", func(s signal) []byte { return aiff(s, "", 16) }, 0.01},
		{"aiff 24-bit", func(s signal) []byte { return aiff(s, "", 24) }, 0.001},
		{"aiff 8-bit", func(s signal) []byte { return aiff(s, "", 8) }, 0.2},
		{"aifc NONE", func(s signal) []byte { return aiff(s, "NONE", 24) }, 0.001},
		{"aifc sowt", func(s signal) []byte { return aiff(s, "sowt", 16) }, 0.01},
		{"aifc fl32", func(s signal) []byte { return aiff(s, "fl32", 32) }, 0.001},
		{"aifc fl64", func(s signal) []byte { return aiff(s, "fl64", 64) }, 0.001},
	}
}

func TestFindEncodings(t *testing.T) {
	for _, e := range encodings() {
		t.Run(e.name, func(t *testing.T) {
			got, err := find(bytes.NewReader(e.build(testSignal())), DefaultThreshold)
			if err != nil {
				t.Fatal(err)
			}
			checkPeaks(t, got, wantDefault, e.tol)
		})
	}
}

func checkPeaks(t *testing.T, got, want []Peak, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d peaks %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i].Time != want[i].Time || math.Abs(got[i].Level-want[i].Level) > tol {
			t.Errorf("peak %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestFindThreshold(t *testing.T) {
	data := wav(testSignal(), wavFloat, 32, false)
	got, err := find(bytes.NewReader(data), -10)
	if err != nil {
		t.Fatal(err)
	}
	checkPeaks(t, got, append(wantDefault, Peak{Time: 2000 * time.Millisecond, Level: 20 * math.Log10(0.4)}), 0.001)

	got, err = find(bytes.NewReader(data), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("0 dB: got %v, want none", got)
	}
}

func TestFindHold(t *testing.T) {
	s := make(signal, rate)
	for i := range s {
		s[i] = []float64{0}
	}
	s[100] = []float64{0.9}
	s[200] = []float64{0.9} // exactly Hold after: same peak
	s[301] = []float64{0.9} // Hold + 1 after: a new one
	got, err := find(bytes.NewReader(wav(s, wavPCM, 16, false)), DefaultThreshold)
	if err != nil {
		t.Fatal(err)
	}
	checkPeaks(t, got, []Peak{
		{Time: 100 * time.Millisecond, Level: 20 * math.Log10(0.9)},
		{Time: 301 * time.Millisecond, Level: 20 * math.Log10(0.9)},
	}, 0.01)
}

// Chunks may come in any order, and odd-sized chunks are padded.
func TestFindChunkOrder(t *testing.T) {
	s := testSignal()
	body := pcm(s, 16, binary.LittleEndian)
	var b bytes.Buffer
	b.WriteString("RIFF\x00\x00\x00\x00WAVE")
	writeChunk(&b, binary.LittleEndian, "LIST", []byte("odd")) // padded to 4
	writeChunk(&b, binary.LittleEndian, "data", body)
	writeChunk(&b, binary.LittleEndian, "fmt ", fmtChunk(wavPCM, 2, 16, false))
	got, err := find(bytes.NewReader(b.Bytes()), DefaultThreshold)
	if err != nil {
		t.Fatal(err)
	}
	checkPeaks(t, got, wantDefault, 0.01)
}

func TestFindErrors(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"not audio": []byte("this is not a wav file, just text"),
		"no data": func() []byte {
			var b bytes.Buffer
			b.WriteString("RIFF\x00\x00\x00\x00WAVE")
			writeChunk(&b, binary.LittleEndian, "fmt ", fmtChunk(wavPCM, 2, 16, false))
			return b.Bytes()
		}(),
		"no fmt": func() []byte {
			var b bytes.Buffer
			b.WriteString("RIFF\x00\x00\x00\x00WAVE")
			writeChunk(&b, binary.LittleEndian, "data", make([]byte, 8))
			return b.Bytes()
		}(),
		"compressed wav": func() []byte {
			var b bytes.Buffer
			b.WriteString("RIFF\x00\x00\x00\x00WAVE")
			writeChunk(&b, binary.LittleEndian, "fmt ", fmtChunk(0x55, 2, 16, false)) // mp3
			writeChunk(&b, binary.LittleEndian, "data", make([]byte, 8))
			return b.Bytes()
		}(),
		"compressed aifc": aiff(testSignal(), "ima4", 16),
	} {
		if _, err := find(bytes.NewReader(data), DefaultThreshold); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestFindFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.aif")
	if err := os.WriteFile(path, aiff(testSignal(), "", 24), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Find(path, DefaultThreshold)
	if err != nil {
		t.Fatal(err)
	}
	checkPeaks(t, got, wantDefault, 0.001)

	if _, err := Find(filepath.Join(t.TempDir(), "missing.wav"), DefaultThreshold); err == nil {
		t.Error("missing file: want error")
	}
}

func TestExtended(t *testing.T) {
	for _, r := range []float64{8000, 22050, 44100, 48000, 96000, 192000} {
		if got := extended(ext80(uint64(r))); got != r {
			t.Errorf("extended(%v) = %v", r, got)
		}
	}
}

func TestParseTimeSig(t *testing.T) {
	for in, want := range map[string]TimeSig{
		"4/4": {4, 4}, "3/4": {3, 4}, "6/8": {6, 8}, "7/8": {7, 8}, "12/8": {12, 8}, "5/4": {5, 4}, " 2 / 2 ": {2, 2},
	} {
		got, err := ParseTimeSig(in)
		if err != nil || got != want {
			t.Errorf("ParseTimeSig(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "4", "4/", "/4", "0/4", "4/0", "4/3", "4/6", "a/b", "-4/4", "4/4/4"} {
		if _, err := ParseTimeSig(in); err == nil {
			t.Errorf("ParseTimeSig(%q): want error", in)
		}
	}
}

func TestBar(t *testing.T) {
	sec := func(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
	for _, tc := range []struct {
		t    time.Duration
		bpm  float64
		ts   TimeSig
		want int
	}{
		// 120 bpm 4/4: a bar is 2s.
		{0, 120, TimeSig{4, 4}, 1},
		{sec(1.999), 120, TimeSig{4, 4}, 1},
		{sec(2), 120, TimeSig{4, 4}, 2},
		{sec(61), 120, TimeSig{4, 4}, 31},
		// 120 bpm 3/4: 1.5s.
		{sec(1.5), 120, TimeSig{3, 4}, 2},
		{sec(2.9), 120, TimeSig{3, 4}, 2},
		// 120 bpm 6/8: three quarter notes, 1.5s, same as 3/4.
		{sec(1.5), 120, TimeSig{6, 8}, 2},
		// 90 bpm 7/8: 3.5 quarter notes, 2.333s.
		{sec(2.3), 90, TimeSig{7, 8}, 1},
		{sec(2.34), 90, TimeSig{7, 8}, 2},
	} {
		if got := Bar(tc.t, tc.bpm, tc.ts); got != tc.want {
			t.Errorf("Bar(%v, %v, %v) = %d, want %d", tc.t, tc.bpm, tc.ts, got, tc.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                          "00:00:00.000",
		1234 * time.Millisecond:    "00:00:01.234",
		1234567 * time.Microsecond: "00:00:01.235",
		(3*time.Hour + 25*time.Minute + 7*time.Second + 89*time.Millisecond): "03:25:07.089",
		59*time.Minute + 59*time.Second + 999600*time.Microsecond:            "01:00:00.000",
	} {
		if got := FormatTime(d); got != want {
			t.Errorf("FormatTime(%v) = %q, want %q", d, got, want)
		}
	}
}

// --- fixture encoders ---

func writeChunk(b *bytes.Buffer, order binary.ByteOrder, id string, body []byte) {
	b.WriteString(id)
	_ = binary.Write(b, order, uint32(len(body)))
	b.Write(body)
	if len(body)%2 == 1 {
		b.WriteByte(0)
	}
}

func fmtChunk(code uint16, channels, bitDepth int, extensible bool) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	tag := code
	if extensible {
		tag = wavExtensible
	}
	width := bitDepth / 8
	_ = binary.Write(&b, le, tag)
	_ = binary.Write(&b, le, uint16(channels))
	_ = binary.Write(&b, le, uint32(rate))
	_ = binary.Write(&b, le, uint32(rate*channels*width))
	_ = binary.Write(&b, le, uint16(channels*width))
	_ = binary.Write(&b, le, uint16(bitDepth))
	if extensible {
		_ = binary.Write(&b, le, uint16(22))
		_ = binary.Write(&b, le, uint16(bitDepth))
		_ = binary.Write(&b, le, uint32(3))
		_ = binary.Write(&b, le, code)
		b.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71})
	}
	return b.Bytes()
}

func wav(s signal, code uint16, bitDepth int, extensible bool) []byte {
	var body []byte
	switch {
	case code == wavFloat:
		body = floats(s, bitDepth, binary.LittleEndian)
	case bitDepth == 8:
		for _, f := range s {
			for _, v := range f {
				body = append(body, byte(int(math.Round(v*127))+128))
			}
		}
	default:
		body = pcm(s, bitDepth, binary.LittleEndian)
	}
	var b bytes.Buffer
	b.WriteString("RIFF\x00\x00\x00\x00WAVE")
	writeChunk(&b, binary.LittleEndian, "fmt ", fmtChunk(code, len(s[0]), bitDepth, extensible))
	writeChunk(&b, binary.LittleEndian, "data", body)
	return b.Bytes()
}

// aiff encodes s as AIFF, or as AIFF-C when compression is set.
func aiff(s signal, compression string, bitDepth int) []byte {
	var body []byte
	switch compression {
	case "fl32", "fl64":
		body = floats(s, bitDepth, binary.BigEndian)
	case "sowt":
		body = pcm(s, bitDepth, binary.LittleEndian)
	default:
		body = pcm(s, bitDepth, binary.BigEndian)
	}
	be := binary.BigEndian
	var comm bytes.Buffer
	_ = binary.Write(&comm, be, uint16(len(s[0])))
	_ = binary.Write(&comm, be, uint32(len(s)))
	_ = binary.Write(&comm, be, uint16(bitDepth))
	comm.Write(ext80(rate))
	form := "AIFF"
	if compression != "" {
		form = "AIFC"
		comm.WriteString(compression)
		comm.Write([]byte{0, 0}) // empty pascal string name, padded
	}
	var ssnd bytes.Buffer
	_ = binary.Write(&ssnd, be, uint32(4)) // offset: 4 junk bytes before the data
	_ = binary.Write(&ssnd, be, uint32(0))
	ssnd.Write([]byte{9, 9, 9, 9})
	ssnd.Write(body)

	var b bytes.Buffer
	b.WriteString("FORM\x00\x00\x00\x00" + form)
	writeChunk(&b, be, "COMM", comm.Bytes())
	writeChunk(&b, be, "SSND", ssnd.Bytes())
	return b.Bytes()
}

func pcm(s signal, bitDepth int, order binary.ByteOrder) []byte {
	width := bitDepth / 8
	full := math.Ldexp(1, bitDepth-1)
	var out []byte
	for _, f := range s {
		for _, v := range f {
			n := int64(math.Round(v * (full - 1)))
			be := make([]byte, width)
			for i := range width {
				be[width-1-i] = byte(n >> (8 * i))
			}
			if order == binary.LittleEndian {
				for i, j := 0, width-1; i < j; i, j = i+1, j-1 {
					be[i], be[j] = be[j], be[i]
				}
			}
			out = append(out, be...)
		}
	}
	return out
}

func floats(s signal, bitDepth int, order binary.ByteOrder) []byte {
	var b bytes.Buffer
	for _, f := range s {
		for _, v := range f {
			if bitDepth == 32 {
				_ = binary.Write(&b, order, float32(v))
			} else {
				_ = binary.Write(&b, order, v)
			}
		}
	}
	return b.Bytes()
}

func ext80(n uint64) []byte {
	lz := bits.LeadingZeros64(n)
	b := make([]byte, 10)
	binary.BigEndian.PutUint16(b[0:2], uint16(16383+63-lz))
	binary.BigEndian.PutUint64(b[2:10], n<<lz)
	return b
}
