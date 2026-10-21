package utfbom_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/dimchansky/utfbom"
)

func benchmarkSkip(b *testing.B, input []byte) {
	src := bytes.NewReader(input)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.Reset(input)
		r, _ := utfbom.Skip(src)
		if _, err := io.Copy(io.Discard, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSkip_SmallBOM(b *testing.B)   { benchmarkSkip(b, []byte("\xEF\xBB\xBFhello, world")) }
func BenchmarkSkip_SmallNoBOM(b *testing.B) { benchmarkSkip(b, []byte("hello, world")) }

func BenchmarkSkip_64KB(b *testing.B) {
	benchmarkSkip(b, append([]byte("\xEF\xBB\xBF"), bytes.Repeat([]byte("hello, world\n"), 64*1024/13)...))
}
