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
					if sr != sr0 {
						t.Fatal("expected Skip to return the same *utfbom.Reader")
					}
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

// scriptedReader returns the given results in order, then io.EOF.
// Data that does not fit into p is returned by the following calls.
type scriptedReader []readResult

type readResult struct {
	data []byte
	err  error
}

func (r *scriptedReader) Read(p []byte) (int, error) {
	if len(*r) == 0 {
		return 0, io.EOF
	}
	res := &(*r)[0]
	n := copy(p, res.data)
	if res.data = res.data[n:]; len(res.data) > 0 {
		return n, nil
	}
	*r = (*r)[1:]
	return n, res.err
}

func emptyReads(n int) []readResult {
	return make([]readResult, n)
}

var errTest = errors.New("test error")

func TestSkipKeepsErrorAfterEmptyReads(t *testing.T) {
	t.Parallel()

	for _, wantErr := range []error{io.EOF, errTest} {
		r := scriptedReader(append(emptyReads(99), readResult{err: wantErr}))

		output, err := io.ReadAll(utfbom.SkipOnly(&r))
		if len(output) != 0 {
			t.Errorf("expected no output, but got %+#v", output)
		}
		if wantErr == io.EOF {
			wantErr = nil // io.ReadAll reports EOF as success
		}
		if err != wantErr {
			t.Errorf("expected %v error, but got %v", wantErr, err)
		}
	}
}

func TestSkipProgressResetsEmptyReadCount(t *testing.T) {
	t.Parallel()

	var script []readResult
	script = append(script, emptyReads(99)...)
	script = append(script, readResult{data: []byte("\xEF")})
	script = append(script, emptyReads(99)...)
	script = append(script, readResult{data: []byte("\xBB\xBFhi")})
	r := scriptedReader(script)

	sr, enc := utfbom.Skip(&r)
	if enc != utfbom.UTF8 {
		t.Fatalf("expected encoding %v, but got %v", utfbom.UTF8, enc)
	}
	output, err := io.ReadAll(sr)
	if string(output) != "hi" || err != nil {
		t.Fatalf("expected \"hi\" and <nil> error, but got %q and %v", output, err)
	}
}

func TestSkipUTF16WithErrorAfterFullProbe(t *testing.T) {
	t.Parallel()

	inputs := []struct {
		data     []byte
		encoding utfbom.Encoding
	}{
		{[]byte("\xFE\xFF\x00\x41"), utfbom.UTF16BigEndian},
		{[]byte("\xFF\xFE\x41\x00"), utfbom.UTF16LittleEndian},
	}
	for _, in := range inputs {
		schedules := map[string]scriptedReader{
			"error with data": {{data: in.data, err: errTest}},
			"error next read": {{data: in.data}, {err: errTest}},
		}
		for name, r := range schedules {
			r := r
			sr, enc := utfbom.Skip(&r)
			if enc != in.encoding {
				t.Errorf("% X, %s: expected encoding %v, but got %v", in.data, name, in.encoding, enc)
			}
			output, err := io.ReadAll(sr)
			if !bytes.Equal(output, in.data[2:]) {
				t.Errorf("% X, %s: expected to read % X, but got % X", in.data, name, in.data[2:], output)
			}
			if !errors.Is(err, errTest) {
				t.Errorf("% X, %s: expected %v error, but got %v", in.data, name, errTest, err)
			}
		}
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

func TestReader_SmallBuffers(t *testing.T) {
	t.Parallel()

	for _, tc := range testCases {
		for size := 1; size <= 4; size++ {
			sr := utfbom.SkipOnly(&sliceReader{tc.input, tc.inputError})

			var output []byte
			buf := make([]byte, size)
			var err error
			for err == nil {
				var n int
				n, err = sr.Read(buf)
				output = append(output, buf[:n]...)
			}
			if !bytes.Equal(output, tc.output) {
				t.Errorf("test %s, buffer %d: expected to read %+#v, but got %+#v", tc.name, size, tc.output, output)
			}
			if wantErr := tc.inputError; wantErr == nil && err != io.EOF || wantErr != nil && !errors.Is(err, wantErr) {
				t.Errorf("test %s, buffer %d: unexpected error %v", tc.name, size, err)
			}
		}
	}
}

// FuzzSkip checks BOM detection and payload preservation for streams that end with io.EOF,
// read whole or one byte at a time. Error schedules are covered by the explicit tests above.
func FuzzSkip(f *testing.F) {
	// Independent of the package: longest BOMs first, so FF FE 00 00 is UTF-32LE.
	boms := []struct {
		bom []byte
		enc utfbom.Encoding
	}{
		{[]byte{0x00, 0x00, 0xFE, 0xFF}, utfbom.UTF32BigEndian},
		{[]byte{0xFF, 0xFE, 0x00, 0x00}, utfbom.UTF32LittleEndian},
		{[]byte{0xEF, 0xBB, 0xBF}, utfbom.UTF8},
		{[]byte{0xFE, 0xFF}, utfbom.UTF16BigEndian},
		{[]byte{0xFF, 0xFE}, utfbom.UTF16LittleEndian},
	}

	for _, tc := range testCases {
		f.Add(tc.input, false)
	}
	f.Add([]byte("\xEF\xBB\xBF\xEF\xBB\xBFhi"), true)
	f.Add([]byte("hi\xEF\xBB\xBF"), true)

	f.Fuzz(func(t *testing.T, input []byte, oneByte bool) {
		wantEnc, wantOut := utfbom.Unknown, input
		for _, b := range boms {
			if bytes.HasPrefix(input, b.bom) {
				wantEnc, wantOut = b.enc, input[len(b.bom):]
				break
			}
		}

		var r io.Reader = bytes.NewReader(input)
		if oneByte {
			r = iotest.OneByteReader(r)
		}
		sr, enc := utfbom.Skip(r)
		output, err := io.ReadAll(sr)
		if enc != wantEnc || !bytes.Equal(output, wantOut) || err != nil {
			t.Fatalf("Skip(% X): got %v, % X, %v; want %v, % X, <nil>", input, enc, output, err, wantEnc, wantOut)
		}
	})
}

func TestEncoding_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		enc  utfbom.Encoding
		want string
	}{
		{utfbom.Unknown, "Unknown"},
		{utfbom.UTF8, "UTF8"},
		{utfbom.UTF16BigEndian, "UTF16BigEndian"},
		{utfbom.UTF16LittleEndian, "UTF16LittleEndian"},
		{utfbom.UTF32BigEndian, "UTF32BigEndian"},
		{utfbom.UTF32LittleEndian, "UTF32LittleEndian"},
		{utfbom.Encoding(-1), "Unknown"},
		{utfbom.Encoding(999), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.enc.String(); got != tt.want {
			t.Errorf("Encoding(%d).String() = %q, want %q", int(tt.enc), got, tt.want)
		}
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
