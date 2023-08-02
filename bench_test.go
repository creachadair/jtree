// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/jwcc"
	"github.com/tailscale/hujson"
)

// A local file path or a URL. For example:
// https://raw.githubusercontent.com/prust/wikipedia-movie-data/master/movies.json
var inputPath = flag.String("input", "testdata/input.json", "Input JSON file path or URL")

func readInput() ([]byte, error) {
	if strings.HasPrefix(*inputPath, "http://") || strings.HasPrefix(*inputPath, "https://") {
		rsp, err := http.Get(*inputPath)
		if err != nil {
			return nil, err
		}
		defer rsp.Body.Close()
		return io.ReadAll(rsp.Body)
	}
	return os.ReadFile(*inputPath)
}

func BenchmarkScanner(b *testing.B) {
	input, err := readInput()
	if err != nil {
		b.Fatalf("Reading test input: %v", err)
	}
	b.Logf("Benchmark input: %d bytes", len(input))

	b.Run("Std", func(b *testing.B) {
		b.Run("Unmarshal", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var ignore any
				if err := json.Unmarshal(input, &ignore); err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})

		b.Run("Tokenize", func(b *testing.B) {
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

		b.Run("Decode", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dec := json.NewDecoder(bytes.NewReader(input))
				var ignore any
				if err := dec.Decode(&ignore); err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	})

	b.Run("HuJSON", func(b *testing.B) {
		b.Run("Parse", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := hujson.Parse(input)
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})

		b.Run("Standardize", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := hujson.Standardize(input)
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	})

	b.Run("JTree", func(b *testing.B) {
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
				}
			}
		})

		b.Run("Stream", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dec := jtree.NewStream(bytes.NewReader(input))
				if err := dec.Parse(noopHandler{}); err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})

		b.Run("ParseAST", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := ast.Parse(bytes.NewReader(input))
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	})

	b.Run("JWCC", func(b *testing.B) {
		b.Run("ParseAST", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				p := ast.NewParser(bytes.NewReader(input))
				p.AllowJWCC(true)
				for {
					_, err := p.Parse()
					if err == io.EOF {
						break
					} else if err != nil {
						b.Fatalf("Unexpected error: %v", err)
					}
				}
			}
		})

		b.Run("ParseJWCC", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := jwcc.Parse(bytes.NewReader(input))
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	})
}

type noopHandler struct{}

func (noopHandler) BeginObject(jtree.Anchor) error        { return nil }
func (noopHandler) EndObject(jtree.Anchor) error          { return nil }
func (noopHandler) BeginArray(jtree.Anchor) error         { return nil }
func (noopHandler) EndArray(jtree.Anchor) error           { return nil }
func (noopHandler) BeginMember(jtree.Anchor) error        { return nil }
func (noopHandler) EndMember(jtree.Anchor) error          { return nil }
func (noopHandler) Value(jtree.Anchor) error              { return nil }
func (noopHandler) SyntaxError(jtree.Anchor, error) error { return nil }
func (noopHandler) EndOfInput(jtree.Anchor)               {}
