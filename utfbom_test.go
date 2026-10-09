package utfbom_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"testing/iotest"
	"time"

	"github.com/dimchansky/utfbom"
)

var testCases = []struct {
	name       string
	input      []byte
	inputError error
	encoding   utfbom.Encoding
	output     []byte
}{
	{"1", []byte{}, nil, utfbom.Unknown, []byte{}},
	{"2", []byte("hello"), nil, utfbom.Unknown, []byte("hello")},
	{"3", []byte("\xEF\xBB\xBF"), nil, utfbom.UTF8, []byte{}},
	{"4", []byte("\xEF\xBB\xBFhello"), nil, utfbom.UTF8, []byte("hello")},
	{"5", []byte("\xFE\xFF"), nil, utfbom.UTF16BigEndian, []byte{}},
	{"6", []byte("\xFF\xFE"), nil, utfbom.UTF16LittleEndian, []byte{}},
	{"7", []byte("\x00\x00\xFE\xFF"), nil, utfbom.UTF32BigEndian, []byte{}},
	{"8", []byte("\xFF\xFE\x00\x00"), nil, utfbom.UTF32LittleEndian, []byte{}},
	{
		"5", []byte("\xFE\xFF\x00\x68\x00\x65\x00\x6C\x00\x6C\x00\x6F"), nil,
		utfbom.UTF16BigEndian,
		[]byte{0x00, 0x68, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F},
	},
	{
		"6", []byte("\xFF\xFE\x68\x00\x65\x00\x6C\x00\x6C\x00\x6F\x00"), nil,
		utfbom.UTF16LittleEndian,
		[]byte{0x68, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00},
	},
	{
		"7", []byte("\x00\x00\xFE\xFF\x00\x00\x00\x68\x00\x00\x00\x65\x00\x00\x00\x6C\x00\x00\x00\x6C\x00\x00\x00\x6F"), nil,
		utfbom.UTF32BigEndian,
		[]byte{0x00, 0x00, 0x00, 0x68, 0x00, 0x00, 0x00, 0x65, 0x00, 0x00, 0x00, 0x6C, 0x00, 0x00, 0x00, 0x6C, 0x00, 0x00, 0x00, 0x6F},
	},
	{
		"8", []byte("\xFF\xFE\x00\x00\x68\x00\x00\x00\x65\x00\x00\x00\x6C\x00\x00\x00\x6C\x00\x00\x00\x6F\x00\x00\x00"), nil,
		utfbom.UTF32LittleEndian,
		[]byte{0x68, 0x00, 0x00, 0x00, 0x65, 0x00, 0x00, 0x00, 0x6C, 0x00, 0x00, 0x00, 0x6C, 0x00, 0x00, 0x00, 0x6F, 0x00, 0x00, 0x00},
	},
	{"9", []byte("\xEF"), nil, utfbom.Unknown, []byte("\xEF")},
	{"10", []byte("\xEF\xBB"), nil, utfbom.Unknown, []byte("\xEF\xBB")},
	{"11", []byte("\xEF\xBB\xBF"), io.ErrClosedPipe, utfbom.UTF8, []byte{}},
	{"12", []byte("\xFE\xFF"), io.ErrClosedPipe, utfbom.Unknown, []byte("\xFE\xFF")},
	{"13", []byte("\xFE"), io.ErrClosedPipe, utfbom.Unknown, []byte("\xFE")},
	{"14", []byte("\xFF\xFE"), io.ErrClosedPipe, utfbom.Unknown, []byte("\xFF\xFE")},
	{"15", []byte("\x00\x00\xFE\xFF"), io.ErrClosedPipe, utfbom.UTF32BigEndian, []byte{}},
	{"16", []byte("\x00\x00\xFE"), io.ErrClosedPipe, utfbom.Unknown, []byte{0x00, 0x00, 0xFE}},
	{"17", []byte("\x00\x00"), io.ErrClosedPipe, utfbom.Unknown, []byte{0x00, 0x00}},
	{"18", []byte("\x00"), io.ErrClosedPipe, utfbom.Unknown, []byte{0x00}},
	{"19", []byte("\xFF\xFE\x00\x00"), io.ErrClosedPipe, utfbom.UTF32LittleEndian, []byte{}},
	{"20", []byte("\xFF\xFE\x00"), io.ErrClosedPipe, utfbom.Unknown, []byte{0xFF, 0xFE, 0x00}},
	{"21", []byte("\xFF\xFE"), io.ErrClosedPipe, utfbom.Unknown, []byte{0xFF, 0xFE}},
	{"22", []byte("\xFF"), io.ErrClosedPipe, utfbom.Unknown, []byte{0xFF}},
	{"23", []byte("\x68\x65"), nil, utfbom.Unknown, []byte{0x68, 0x65}},
	{"24", []byte("\ufeff"), nil, utfbom.UTF8, []byte{}},
	{"25", []byte("\ufeffhello"), nil, utfbom.UTF8, []byte("hello")},
	{"26", []byte("hello\ufeffworld"), nil, utfbom.Unknown, []byte("hello\ufeffworld")},
	{"27", []byte("\ufeffhello\ufeffworld"), nil, utfbom.UTF8, []byte("hello\ufeffworld")},
}

type sliceReader struct {
	input      []byte
	inputError error
}

func (r *sliceReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	if err = r.getError(); err != nil {
		return
	}

	n = copy(p, r.input)
	r.input = r.input[n:]
	err = r.getError()
	return
}

func (r *sliceReader) getError() (err error) {
	if len(r.input) == 0 {
		if r.inputError == nil {
			err = io.EOF
		} else {
			err = r.inputError
		}
	}
	return
}

var readMakers = []struct {
	name string
	fn   func(io.Reader) io.Reader
}{
	{"full", func(r io.Reader) io.Reader { return r }},
	{"byte", iotest.OneByteReader},
}

func TestSkip(t *testing.T) {
	t.Parallel()

	for _, tc := range testCases {
		tc := tc
		t.Run("test "+tc.name, func(t *testing.T) {
			t.Parallel()

			for _, readMaker := range readMakers {
				readMaker := readMaker
				t.Run("reader="+readMaker.name, func(t *testing.T) {
					t.Parallel()

					r := readMaker.fn(&sliceReader{tc.input, tc.inputError})

					sr, enc := utfbom.Skip(r)
					if enc != tc.encoding {
						t.Fatalf("expected encoding %v, but got %v", tc.encoding, enc)
					}

					output, err := io.ReadAll(sr)
					if !reflect.DeepEqual(output, tc.output) {
						t.Fatalf("expected to read %+#v, but got %+#v", tc.output, output)
					}
					if !errors.Is(err, tc.inputError) {
						t.Fatalf("expected to get %+#v error, but got %+#v", tc.inputError, err)
					}
				})
			}
		})
	}
}

func TestSkipSkip(t *testing.T) {
	t.Parallel()

	for _, tc := range testCases {
		tc := tc
		t.Run("test "+tc.name, func(t *testing.T) {
			t.Parallel()

			for _, readMaker := range readMakers {
				readMaker := readMaker
				t.Run("reader="+readMaker.name, func(t *testing.T) {
					t.Parallel()

					r := readMaker.fn(&sliceReader{tc.input, tc.inputError})

					sr0, _ := utfbom.Skip(r)
					sr, enc := utfbom.Skip(sr0)
					if enc != tc.encoding {
						t.Fatalf("expected encoding %v, but got %v", tc.encoding, enc)
					}

					output, err := io.ReadAll(sr)
					if !reflect.DeepEqual(output, tc.output) {
						t.Fatalf("expected to read %+#v, but got %+#v", tc.output, output)
					}
					if !errors.Is(err, tc.inputError) {
						t.Fatalf("expected to get %+#v error, but got %+#v", tc.inputError, err)
					}
				})
			}
		})
	}
}

func TestSkipOnly(t *testing.T) {
	t.Parallel()

	for _, tc := range testCases {
		tc := tc
		t.Run("test "+tc.name, func(t *testing.T) {
			t.Parallel()

			for _, readMaker := range readMakers {
				readMaker := readMaker
				t.Run("reader="+readMaker.name, func(t *testing.T) {
					t.Parallel()

					r := readMaker.fn(&sliceReader{tc.input, tc.inputError})

					sr := utfbom.SkipOnly(r)

					output, err := io.ReadAll(sr)
					if !reflect.DeepEqual(output, tc.output) {
						t.Fatalf("expected to read %+#v, but got %+#v", tc.output, output)
					}
					if !errors.Is(err, tc.inputError) {
						t.Fatalf("expected to get %+#v error, but got %+#v", tc.inputError, err)
					}
				})
			}
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	return 0, nil
}

type readerEncoding struct {
	Rd  *utfbom.Reader
	Enc utfbom.Encoding
}

func TestSkipZeroReader(t *testing.T) {
	t.Parallel()

	var z zeroReader

	c := make(chan readerEncoding)
	go func() {
		r, enc := utfbom.Skip(z)
		c <- readerEncoding{r, enc}
	}()

	select {
	case re := <-c:
		if re.Enc != utfbom.Unknown {
			t.Error("utfbom.Unknown encoding expected")
		} else {
			var b [1]byte
			n, err := re.Rd.Read(b[:])
			if n != 0 {
				t.Error("unexpected bytes count:", n)
			}
			if err != io.ErrNoProgress {
				t.Error("unexpected error:", err)
			}
		}
	case <-time.After(time.Second):
		t.Error("test timed out (endless loop in utfbom.Skip?)")
	}
}

func TestSkipOnlyZeroReader(t *testing.T) {
	t.Parallel()

	var z zeroReader

	c := make(chan *utfbom.Reader)
	go func() {
		r := utfbom.SkipOnly(z)
		c <- r
	}()

	select {
	case r := <-c:
		var b [1]byte
		n, err := r.Read(b[:])
		if n != 0 {
			t.Error("unexpected bytes count:", n)
		}
		if err != io.ErrNoProgress {
			t.Error("unexpected error:", err)
		}
	case <-time.After(time.Second):
		t.Error("test timed out (endless loop in utfbom.Skip?)")
	}
}

func TestReader_ReadEmpty(t *testing.T) {
	t.Parallel()

	for _, tc := range testCases {
		tc := tc
		t.Run("test "+tc.name, func(t *testing.T) {
			t.Parallel()

			for _, readMaker := range readMakers {
				readMaker := readMaker
				t.Run("reader="+readMaker.name, func(t *testing.T) {
					t.Parallel()

					r := readMaker.fn(&sliceReader{tc.input, tc.inputError})

					sr := utfbom.SkipOnly(r)

					n, err := sr.Read(nil)
					if n != 0 {
						t.Fatalf("test %v reader=%s: expected to read zero bytes, but got %v", tc.name, readMaker.name, n)
					}
					if err != nil {
						t.Fatalf("test %v reader=%s: expected to get <nil> error, but got %+#v", tc.name, readMaker.name, err)
					}
				})
			}
		})
	}
}

func TestEncoding_String(t *testing.T) {
	t.Parallel()

	for e := utfbom.Unknown; e <= utfbom.UTF32LittleEndian; e++ {
		s := e.String()
		if s == "" {
			t.Errorf("no string for %#v", e)
		}
	}

	s := utfbom.Encoding(999).String()
	if s != "Unknown" {
		t.Errorf("wrong string '%s' for invalid encoding", s)
	}
}

func TestBOMConstant(t *testing.T) {
	t.Parallel()

	if utfbom.BOM != '\ufeff' {
		t.Fatalf("expected utfbom.BOM to be '\\ufeff', got %q", utfbom.BOM)
	}
	if string(utfbom.BOM) != "\ufeff" {
		t.Fatalf("expected string(utfbom.BOM) to be \"\\ufeff\", got %q", string(utfbom.BOM))
	}
	if !bytes.Equal([]byte(string(utfbom.BOM)), []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("expected BOM UTF-8 bytes to be [0xEF, 0xBB, 0xBF], got %v", []byte(string(utfbom.BOM)))
	}
}

func TestBOMRuneRemoval(t *testing.T) {
	t.Parallel()

	testInputs := []struct {
		name     string
		input    string
		expected string
		enc      utfbom.Encoding
	}{
		{"empty with BOM", "\ufeff", "", utfbom.UTF8},
		{"word with BOM", "\ufeffhello", "hello", utfbom.UTF8},
		{"BOM constant prefix", string(utfbom.BOM) + "world", "world", utfbom.UTF8},
		{"no BOM", "plain text", "plain text", utfbom.Unknown},
		{"BOM in middle preserved", "hello\ufeffworld", "hello\ufeffworld", utfbom.Unknown},
		{"leading BOM stripped middle preserved", "\ufeffhello\ufeffworld", "hello\ufeffworld", utfbom.UTF8},
	}

	for _, tc := range testInputs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sr, enc := utfbom.Skip(bytes.NewReader([]byte(tc.input)))
			if enc != tc.enc {
				t.Fatalf("expected encoding %v, got %v", tc.enc, enc)
			}
			out, err := io.ReadAll(sr)
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if string(out) != tc.expected {
				t.Fatalf("expected output %q, got %q", tc.expected, string(out))
			}

			so := utfbom.SkipOnly(bytes.NewReader([]byte(tc.input)))
			out, err = io.ReadAll(so)
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if string(out) != tc.expected {
				t.Fatalf("expected output %q, got %q", tc.expected, string(out))
			}
		})
	}
}

func TestBOMRuneRemoval_AllEncodings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       []byte
		expectedEnc utfbom.Encoding
		expectedOut []byte
	}{
		{
			name:        "UTF8 with BOM",
			input:       []byte("\ufeffsample text"),
			expectedEnc: utfbom.UTF8,
			expectedOut: []byte("sample text"),
		},
		{
			name:        "UTF16-BE with BOM",
			input:       []byte{0xFE, 0xFF, 0x00, 'A', 0x00, 'B'},
			expectedEnc: utfbom.UTF16BigEndian,
			expectedOut: []byte{0x00, 'A', 0x00, 'B'},
		},
		{
			name:        "UTF16-LE with BOM",
			input:       []byte{0xFF, 0xFE, 'A', 0x00, 'B', 0x00},
			expectedEnc: utfbom.UTF16LittleEndian,
			expectedOut: []byte{'A', 0x00, 'B', 0x00},
		},
		{
			name:        "UTF32-BE with BOM",
			input:       []byte{0x00, 0x00, 0xFE, 0xFF, 0x00, 0x00, 0x00, 'A'},
			expectedEnc: utfbom.UTF32BigEndian,
			expectedOut: []byte{0x00, 0x00, 0x00, 'A'},
		},
		{
			name:        "UTF32-LE with BOM",
			input:       []byte{0xFF, 0xFE, 0x00, 0x00, 'A', 0x00, 0x00, 0x00},
			expectedEnc: utfbom.UTF32LittleEndian,
			expectedOut: []byte{'A', 0x00, 0x00, 0x00},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			sr, enc := utfbom.Skip(bytes.NewReader(c.input))
			if enc != c.expectedEnc {
				t.Fatalf("expected encoding %v, got %v", c.expectedEnc, enc)
			}
			if sr.Encoding() != c.expectedEnc {
				t.Fatalf("expected Reader.Encoding() %v, got %v", c.expectedEnc, sr.Encoding())
			}
			out, err := io.ReadAll(sr)
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if !bytes.Equal(out, c.expectedOut) {
				t.Fatalf("expected %v, got %v", c.expectedOut, out)
			}
		})
	}
}

func TestBOMRuneRemoval_IncrementalReads(t *testing.T) {
	t.Parallel()

	input := []byte("\ufeffstream-content-without-bom")
	expected := []byte("stream-content-without-bom")

	for bufSize := 1; bufSize <= 16; bufSize++ {
		bufSize := bufSize
		t.Run(fmt.Sprintf("bufSize=%d", bufSize), func(t *testing.T) {
			t.Parallel()

			sr := utfbom.SkipOnly(bytes.NewReader(input))
			var result []byte
			buf := make([]byte, bufSize)
			for {
				n, err := sr.Read(buf)
				if n > 0 {
					result = append(result, buf[:n]...)
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if !bytes.Equal(result, expected) {
				t.Fatalf("expected %q, got %q", string(expected), string(result))
			}
		})
	}
}

func TestReader_Encoding(t *testing.T) {
	t.Parallel()

	sr, _ := utfbom.Skip(bytes.NewReader([]byte("\ufeffhello")))
	if sr.Encoding() != utfbom.UTF8 {
		t.Fatalf("expected UTF8, got %v", sr.Encoding())
	}

	so := utfbom.SkipOnly(bytes.NewReader([]byte{0xFE, 0xFF, 0x00, 0x41}))
	if so.Encoding() != utfbom.UTF16BigEndian {
		t.Fatalf("expected UTF16BigEndian, got %v", so.Encoding())
	}
}

func ExampleSkipOnly() {
	byteData := []byte("\xEF\xBB\xBFhello")

	fmt.Println("Input:", byteData)

	output, err := io.ReadAll(utfbom.SkipOnly(bytes.NewReader(byteData)))
	if err != nil {
		panic(err)
	}

	fmt.Println("ReadAll with BOM skipping", output)

	// Output:
	// Input: [239 187 191 104 101 108 108 111]
	// ReadAll with BOM skipping [104 101 108 108 111]
}

func ExampleSkip() {
	byteData := []byte("\xEF\xBB\xBFhello")

	sr, enc := utfbom.Skip(bytes.NewReader(byteData))

	fmt.Printf("Detected encoding: %s\n", enc)

	output, err := io.ReadAll(sr)
	if err != nil {
		panic(err)
	}

	fmt.Println("ReadAll with BOM detection and skipping", output)

	// Output:
	// Detected encoding: UTF8
	// ReadAll with BOM detection and skipping [104 101 108 108 111]
}
