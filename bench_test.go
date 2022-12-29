// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/ast"
)

var inputPath = flag.String("input", "testdata/input.json", "Input JSON file")

func BenchmarkScanner(b *testing.B) {
	input, err := os.ReadFile(*inputPath)
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
