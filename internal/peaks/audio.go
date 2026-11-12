package peaks

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// format describes how the sample data of an uncompressed audio file is laid
// out. Only the fields Find needs: it never reconstructs the waveform, it
// only looks at each frame's loudest sample.
type format struct {
	sampleRate float64
	channels   int
	width      int  // bytes per sample, per channel
	float      bool // IEEE float rather than signed integer PCM
	bigEndian  bool
	unsigned8  bool // 8-bit WAV is unsigned, offset by 128
}

// stream is the sample data of a file, with its layout.
type stream struct {
	format
	data io.Reader
}

// open parses the header of a WAV or AIFF/AIFF-C file and returns a stream
// positioned at its sample data. The container is detected from the file's
// magic bytes, not its extension.
func open(r io.ReadSeeker) (*stream, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.New("not a WAV or AIFF file")
		}
		return nil, fmt.Errorf("reading header: %w", err)
	}
	switch {
	case string(hdr[0:4]) == "RIFF" && string(hdr[8:12]) == "WAVE":
		return openWAV(r)
	case string(hdr[0:4]) == "FORM" && (string(hdr[8:12]) == "AIFF" || string(hdr[8:12]) == "AIFC"):
		return openAIFF(r, string(hdr[8:12]) == "AIFC")
	default:
		return nil, errors.New("not a WAV or AIFF file")
	}
}

// chunk is one RIFF/IFF chunk header: where its body starts and how long it is.
type chunk struct {
	id   string
	size int64
	body int64 // offset of the body in the file
}

// chunks walks the chunk list after the 12-byte container header, calling fn
// for each. fn may read from r; chunks re-seeks to the next chunk afterwards,
// so the order of chunks in the file doesn't matter.
func chunks(r io.ReadSeeker, order binary.ByteOrder, fn func(chunk) error) error {
	pos := int64(12)
	for {
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			return err
		}
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil // end of file, or trailing junk shorter than a header
			}
			return err
		}
		c := chunk{id: string(hdr[0:4]), size: int64(order.Uint32(hdr[4:8])), body: pos + 8}
		if err := fn(c); err != nil {
			return err
		}
		pos = c.body + c.size + c.size%2 // chunks are padded to an even length
	}
}

const (
	wavPCM        = 1
	wavFloat      = 3
	wavExtensible = 0xFFFE
)

func openWAV(r io.ReadSeeker) (*stream, error) {
	var f *format
	var data *chunk
	err := chunks(r, binary.LittleEndian, func(c chunk) error {
		switch c.id {
		case "fmt ":
			if c.size < 16 {
				return errors.New("fmt chunk too short")
			}
			b := make([]byte, min(c.size, 40))
			if _, err := io.ReadFull(r, b); err != nil {
				return fmt.Errorf("reading fmt chunk: %w", err)
			}
			le := binary.LittleEndian
			code := le.Uint16(b[0:2])
			if code == wavExtensible {
				if len(b) < 26 {
					return errors.New("extensible fmt chunk too short")
				}
				code = le.Uint16(b[24:26]) // first two bytes of the SubFormat GUID
			}
			channels := int(le.Uint16(b[2:4]))
			blockAlign := int(le.Uint16(b[12:14]))
			bits := int(le.Uint16(b[14:16]))
			if channels == 0 {
				return errors.New("fmt chunk has zero channels")
			}
			width := blockAlign / channels
			if width == 0 {
				width = (bits + 7) / 8
			}
			f = &format{
				sampleRate: float64(le.Uint32(b[4:8])),
				channels:   channels,
				width:      width,
			}
			switch code {
			case wavPCM:
				if width < 1 || width > 4 {
					return fmt.Errorf("unsupported PCM sample size: %d bits", bits)
				}
				f.unsigned8 = width == 1
			case wavFloat:
				if width != 4 && width != 8 {
					return fmt.Errorf("unsupported float sample size: %d bits", bits)
				}
				f.float = true
			default:
				return fmt.Errorf("unsupported WAV encoding (format code %#x); only PCM and float are supported", code)
			}
		case "data":
			data = &c
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return finish(r, f, data, "fmt", "data")
}

func openAIFF(r io.ReadSeeker, aifc bool) (*stream, error) {
	var f *format
	var data *chunk
	err := chunks(r, binary.BigEndian, func(c chunk) error {
		switch c.id {
		case "COMM":
			if c.size < 18 || (aifc && c.size < 22) {
				return errors.New("COMM chunk too short")
			}
			b := make([]byte, 22)
			if _, err := io.ReadFull(r, b[:min(c.size, 22)]); err != nil {
				return fmt.Errorf("reading COMM chunk: %w", err)
			}
			be := binary.BigEndian
			channels := int(be.Uint16(b[0:2]))
			bits := int(be.Uint16(b[6:8]))
			if channels == 0 {
				return errors.New("COMM chunk has zero channels")
			}
			f = &format{
				sampleRate: extended(b[8:18]),
				channels:   channels,
				width:      (bits + 7) / 8,
				bigEndian:  true,
			}
			compression := "NONE"
			if aifc {
				compression = string(b[18:22])
			}
			switch compression {
			case "NONE", "twos":
			case "sowt":
				f.bigEndian = false
			case "fl32", "FL32":
				f.float, f.width = true, 4
			case "fl64", "FL64":
				f.float, f.width = true, 8
			default:
				return fmt.Errorf("unsupported AIFF-C compression %q", compression)
			}
			if !f.float && (f.width < 1 || f.width > 4) {
				return fmt.Errorf("unsupported sample size: %d bits", bits)
			}
		case "SSND":
			if c.size < 8 {
				return errors.New("SSND chunk too short")
			}
			var b [4]byte
			if _, err := io.ReadFull(r, b[:]); err != nil {
				return fmt.Errorf("reading SSND chunk: %w", err)
			}
			// The sample data starts offset bytes after the 8-byte SSND header.
			offset := int64(binary.BigEndian.Uint32(b[:])) + 8
			if offset > c.size {
				return errors.New("SSND offset past end of chunk")
			}
			data = &chunk{id: c.id, size: c.size - offset, body: c.body + offset}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return finish(r, f, data, "COMM", "SSND")
}

func finish(r io.ReadSeeker, f *format, data *chunk, fmtID, dataID string) (*stream, error) {
	if f == nil {
		return nil, fmt.Errorf("no %s chunk", fmtID)
	}
	if data == nil {
		return nil, fmt.Errorf("no %s chunk", dataID)
	}
	if f.sampleRate <= 0 || math.IsInf(f.sampleRate, 0) || math.IsNaN(f.sampleRate) {
		return nil, fmt.Errorf("invalid sample rate %v", f.sampleRate)
	}
	if _, err := r.Seek(data.body, io.SeekStart); err != nil {
		return nil, err
	}
	return &stream{format: *f, data: io.LimitReader(r, data.size)}, nil
}

// extended decodes an 80-bit IEEE 754 extended-precision float, the format
// AIFF stores its sample rate in.
func extended(b []byte) float64 {
	exp := int(binary.BigEndian.Uint16(b[0:2]))
	mant := binary.BigEndian.Uint64(b[2:10])
	sign := 1.0
	if exp&0x8000 != 0 {
		sign = -1
		exp &= 0x7FFF
	}
	if exp == 0 && mant == 0 {
		return 0
	}
	if exp == 0x7FFF {
		return math.NaN()
	}
	return sign * math.Ldexp(float64(mant), exp-16383-63)
}

// frames calls fn with the absolute amplitude of the loudest channel of each
// frame, in order, as a fraction of full scale (1.0 = 0 dBFS). A trailing
// partial frame is ignored.
func (s *stream) frames(fn func(amp float64)) error {
	frameSize := s.width * s.channels
	buf := make([]byte, frameSize*4096)
	br := bufio.NewReaderSize(s.data, len(buf))
	for {
		n, err := io.ReadFull(br, buf)
		n -= n % frameSize
		for off := 0; off < n; off += frameSize {
			peak := 0.0
			for c := range s.channels {
				p := off + c*s.width
				peak = max(peak, math.Abs(s.sample(buf[p:p+s.width])))
			}
			fn(peak)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// sample decodes one sample as a fraction of full scale.
func (s *stream) sample(b []byte) float64 {
	if s.float {
		var order binary.ByteOrder = binary.LittleEndian
		if s.bigEndian {
			order = binary.BigEndian
		}
		if s.width == 4 {
			return float64(math.Float32frombits(order.Uint32(b)))
		}
		return math.Float64frombits(order.Uint64(b))
	}
	if s.unsigned8 {
		return float64(int(b[0])-128) / 128
	}
	// Left-justify the integer in 32 bits so every width shares one scale.
	var u uint32
	for i := range b {
		shift := 24 - 8*i // most significant byte first
		if !s.bigEndian {
			shift = 24 - 8*(len(b)-1-i)
		}
		u |= uint32(b[i]) << shift
	}
	return float64(int32(u)) / (1 << 31)
}
