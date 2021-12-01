package jtree_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/creachadair/jtree"
)

func BenchmarkScanner(b *testing.B) {
	input, err := os.ReadFile("testdata/input.json")
	if err != nil {
		b.Fatalf("Reading test input: %v", err)
	}
	b.Logf("Benchmark input: %d bytes", len(input))

	b.Run("Decoder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dec := json.NewDecoder(bytes.NewReader(input))
			for {
				_, err := dec.Token()
				if err == io.EOF {
					break
				} else if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		}
	})

	b.Run("Scanner", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dec := jtree.NewScanner(bytes.NewReader(input))
			for {
				err := dec.Next()
				if err == io.EOF {
					break
				} else if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}

				// The standard library Decoder converts tokens to values.
				// For a fair comparison, do the same for string and numbers.
				switch dec.Token() {
				case jtree.String:
					dec.Unescape()
				case jtree.Integer:
					dec.Int64()
				case jtree.Number:
					dec.Float64()
				}
			}
		}
	})
}
