// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree_test

import (
	"io"
	"strings"
	"testing"

	"github.com/creachadair/jtree"
	"github.com/google/go-cmp/cmp"
)

func TestScanner(t *testing.T) {
	tests := []struct {
		input string
		want  []jtree.Token
	}{
		// Empty inputs
		{"", nil},
		{"  ", nil},
		{"\n\n  \n", nil},
		{"\t  \r\n \t  \r\n", nil},

		// Constants
		{"true false null", []jtree.Token{jtree.True, jtree.False, jtree.Null}},

		// Punctuation
		{"{ [ ] } , :", []jtree.Token{
			jtree.LBrace, jtree.LSquare, jtree.RSquare, jtree.RBrace, jtree.Comma, jtree.Colon,
		}},

		// Strings
		{`"" "a b c" "a\nb\tc"`, []jtree.Token{jtree.String, jtree.String, jtree.String}},
		{`"\"\\\/\b\f\n\r\t"`, []jtree.Token{jtree.String}},
		{`"\u0000\u01fc\uAA9c"`, []jtree.Token{jtree.String}},

		// Numbers
		{`0 -1 5139 2.3 5e+9 3.6E+4 -0.001E-100`, []jtree.Token{
			jtree.Integer, jtree.Integer, jtree.Integer,
			jtree.Number, jtree.Number, jtree.Number, jtree.Number,
		}},

		// Mixed types
		{`{true,"false":-15 null[]}`, []jtree.Token{
			jtree.LBrace, jtree.True, jtree.Comma, jtree.String, jtree.Colon,
			jtree.Integer, jtree.Null, jtree.LSquare, jtree.RSquare, jtree.RBrace,
		}},
		{`{"a": true, "b":[null, 1, 0.5]}`, []jtree.Token{
			jtree.LBrace,
			jtree.String, jtree.Colon, jtree.True, jtree.Comma,
			jtree.String, jtree.Colon,
			jtree.LSquare,
			jtree.Null, jtree.Comma, jtree.Integer, jtree.Comma, jtree.Number,
			jtree.RSquare,
			jtree.RBrace,
		}},
		{`"a",1,true
       false["b"]
       `, []jtree.Token{
			jtree.String, jtree.Comma, jtree.Integer, jtree.Comma, jtree.True,
			jtree.False, jtree.LSquare, jtree.String, jtree.RSquare,
		}},
	}

	for _, test := range tests {
		var got []jtree.Token
		s := jtree.NewScanner(strings.NewReader(test.input))
		for s.Next() == nil {
			got = append(got, s.Token())
		}
		if s.Err() != io.EOF {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestScanner_decodeAs(t *testing.T) {
	mustScan := func(t *testing.T, input string, want jtree.Token) *jtree.Scanner {
		t.Helper()
		s := jtree.NewScanner(strings.NewReader(input))
		if s.Next() != nil {
			t.Fatalf("Next failed: %v", s.Err())
		} else if s.Token() != want {
			t.Fatalf("Next token: got %v, want %v", s.Token(), want)
		}
		return s
	}

	t.Run("Integer", func(t *testing.T) {
		s := mustScan(t, `-15`, jtree.Integer)
		if got := s.Int64(); got != -15 {
			t.Errorf("Decode int64: got %v, want -15", got)
		}
		if got := s.Float64(); got != -15 {
			t.Errorf("Decode float64: got %v, want -15", got)
		}
	})
	t.Run("Number", func(t *testing.T) {
		s := mustScan(t, `3.25e-5`, jtree.Number)
		if got := s.Float64(); got != 3.25e-5 {
			t.Errorf("Decode float64: got %v, want 3.25e-5", got)
		}
	})
	t.Run("Constants", func(t *testing.T) {
		mustScan(t, `true`, jtree.True)
		mustScan(t, `false`, jtree.False)
		mustScan(t, `null`, jtree.Null)
	})
	t.Run("String", func(t *testing.T) {
		const wantText = `"a\tb\u0020c\n"` // as written, without quotes
		const wantDec = "a\tb c\n"         // with escapes undone
		s := mustScan(t, `"a\tb\u0020c\n"`, jtree.String)
		text := s.Text()
		if got := string(text); got != wantText {
			t.Errorf("Text: got %#q, want %#q", got, wantText)
		}
		if u, err := jtree.Unquote(text[1 : len(text)-1]); err != nil {
			t.Errorf("Unquote failed: %v", err)
		} else if got := string(u); got != wantDec {
			t.Errorf("Unquote: got %#q, want %#q", got, wantDec)
		}
	})
}

func TestQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{" ", " "},
		{"a\t\nb", `a\t\nb`},
		{"\x00\x01\x02", `\u0000\u0001\u0002`},
		{`a "b c\" d"`, `a \"b c\\\" d\"`},
		{`\ufffd`, `\\ufffd`},
		{"\u2028 \u2029 \ufffd", `\u2028 \u2029 \ufffd`},
		{"This is the end\v", `This is the end\u000b`},
		{"<\x1e>", `<\u001e>`},
	}
	for _, test := range tests {
		got := string(jtree.Quote(test.input))
		if got != test.want {
			t.Errorf("Input: %#q\nGot:  %#q\nWant: %#q", test.input, got, test.want)
		}
	}
}
