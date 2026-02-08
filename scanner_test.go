// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree_test

import (
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
		for s.Next() {
			got = append(got, s.Token())
		}
		if s.Err() != nil {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestScanner_withComments(t *testing.T) {
	tests := []struct {
		input string
		want  []jtree.Token
		coms  []string
	}{
		{"/* block comment */\n\n\n", []jtree.Token{jtree.BlockComment},
			[]string{"/* block comment */"}},
		{"// line 1\n\n// line 2\n", []jtree.Token{jtree.LineComment, jtree.LineComment},
			[]string{"// line 1\n", "// line 2\n"}}, // N.B. includes terminating newline, if present
		{"// line at EOF", []jtree.Token{jtree.LineComment},
			[]string{"// line at EOF"}},
		{`{
 "x": 1, // howdy do
 "y" /* hide me */ : 2.0 }`, []jtree.Token{
			jtree.LBrace, jtree.String, jtree.Colon, jtree.Integer, jtree.Comma, jtree.LineComment,
			jtree.String, jtree.BlockComment, jtree.Colon, jtree.Number, jtree.RBrace,
		}, []string{
			"// howdy do\n", "/* hide me */",
		}},

		{`"a" // line
false /*
  this is a comment
*/ 1 null [ {} ]`, []jtree.Token{
			jtree.String, jtree.LineComment, jtree.False, jtree.BlockComment,
			jtree.Integer, jtree.Null, jtree.LSquare, jtree.LBrace, jtree.RBrace, jtree.RSquare,
		}, []string{
			"// line\n", "/*\n  this is a comment\n*/",
		}},

		{"/* x */\n{\n}//foo", []jtree.Token{
			jtree.BlockComment, jtree.LBrace, jtree.RBrace, jtree.LineComment,
		}, []string{
			"/* x */", "//foo",
		}},

		{"/**\n*/", []jtree.Token{jtree.BlockComment}, []string{"/**\n*/"}},

		{`/**/"foo"/***/"bar"/****/"baz"/*****/false/*x*/null`, []jtree.Token{
			jtree.BlockComment, jtree.String,
			jtree.BlockComment, jtree.String,
			jtree.BlockComment, jtree.String,
			jtree.BlockComment, jtree.False,
			jtree.BlockComment, jtree.Null,
		}, []string{
			"/**/", "/***/", "/****/", "/*****/", "/*x*/",
		}},
	}

	for _, test := range tests {
		var got []jtree.Token
		var coms []string
		s := jtree.NewScanner(strings.NewReader(test.input))
		s.AllowComments(true)
		for s.Next() {
			got = append(got, s.Token())
			if tok := s.Token(); tok == jtree.LineComment || tok == jtree.BlockComment {
				coms = append(coms, string(s.Text()))
			}
		}
		if s.Err() != nil {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", test.input, diff)
		}
		if diff := cmp.Diff(test.coms, coms); diff != "" {
			t.Errorf("Input: %#q\nComments: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestScanner_decodeAs(t *testing.T) {
	mustScan := func(t *testing.T, input string, want jtree.Token) *jtree.Scanner {
		t.Helper()
		s := jtree.NewScanner(strings.NewReader(input))
		if !s.Next() {
			t.Fatalf("Next failed: %v", s.Err())
		} else if s.Token() != want {
			t.Fatalf("Next token: got %v, want %v", s.Token(), want)
		}
		return s
	}

	t.Run("Integer", func(t *testing.T) {
		mustScan(t, `-15`, jtree.Integer)
	})
	t.Run("Number", func(t *testing.T) {
		mustScan(t, `3.25e-5`, jtree.Number)
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
		if u, err := jtree.Unquote(text); err != nil {
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
		{"", `""`},
		{" ", `" "`},
		{"a\t\nb", `"a\t\nb"`},
		{"\x00\x01\x02", `"\u0000\u0001\u0002"`},
		{`a "b c\" d"`, `"a \"b c\\\" d\""`},
		{`\ufffd`, `"\\ufffd"`},
		{"\u2028 \u2029 \ufffd", `"\u2028 \u2029 \ufffd"`},
		{"This is the end\v", `"This is the end\u000b"`},
		{"<\x1e>", `"<\u001e>"`},
	}
	for _, test := range tests {
		got := string(jtree.Quote(test.input))
		if got != test.want {
			t.Errorf("Input: %#q\nGot:  %#q\nWant: %#q", test.input, got, test.want)
		}
	}
}

func TestScannerLoc(t *testing.T) {
	type tokPos struct {
		Tok jtree.Token
		Pos string
	}
	tests := []struct {
		input string
		want  []tokPos
	}{
		{"", nil},
		{"{ }", []tokPos{{jtree.LBrace, "1:0-1"}, {jtree.RBrace, "1:2-3"}}},
		{`"foo" // bar`, []tokPos{{jtree.String, "1:0-5"}, {jtree.LineComment, "1:6-12"}}},
		{"/* ok */\ntrue\n false\n", []tokPos{{jtree.BlockComment, "1:0-8"}, {jtree.True, "2:0-4"}, {jtree.False, "3:1-6"}}},
		{"/* abc */", []tokPos{{jtree.BlockComment, "1:0-9"}}},
		{"/* ok\n*/\n null", []tokPos{{jtree.BlockComment, "1:0-2:2"}, {jtree.Null, "3:1-5"}}},
		{"// first\n[1, /*x*/, 2\n]", []tokPos{
			{jtree.LineComment, "1:0-2:0"}, {jtree.LSquare, "2:0-1"}, {jtree.Integer, "2:1-2"},
			{jtree.Comma, "2:2-3"}, {jtree.BlockComment, "2:4-9"}, {jtree.Comma, "2:9-10"},
			{jtree.Integer, "2:11-12"}, {jtree.RSquare, "3:0-1"},
		}},
	}
	for _, tc := range tests {
		var got []tokPos
		s := jtree.NewScanner(strings.NewReader(tc.input))
		s.AllowComments(true)
		for s.Next() {
			got = append(got, tokPos{s.Token(), s.Location().String()})
		}
		if s.Err() != nil {
			t.Errorf("Next failed: %v", s.Err())
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("Input: %#q\nTokens: (-want, +got)\n%s", tc.input, diff)
		}
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		input string
		want  string
		fail  bool
	}{
		{``, ``, true},                        // missing quotes
		{`"missing quote`, ``, true},          // missing quotes
		{`missing quote"`, ``, true},          // missing quotes
		{`""`, ``, false},                     // ok
		{`"ok go"`, "ok go", false},           // ok
		{`"abc\ndef"`, "abc\ndef", false},     // C escapes
		{`"\tabc\n"`, "\tabc\n", false},       // C escapes
		{`"\b\f\n\r\t"`, "\b\f\n\r\t", false}, // C escapes
		{`"a \u0026 b"`, "a & b", false},      // short Unicode escape
		{`"\u"`, ``, true},                    // incomplete Unicode escape
		{`"\u00"`, ``, true},                  // incomplete Unicode escape
		{`"\u00x9"`, "\ufffd", false},         // invalid Unicode escape
		{`"\u019 "`, "\ufffd", false},         // invalid Unicode escape
		{`"a\"b"`, `a"b`, false},              // ok
		{`"a\\b\\cd"`, `a\b\cd`, false},       // ok
	}

	for _, test := range tests {
		got, err := jtree.Unquote([]byte(test.input))
		if err != nil {
			if !test.fail {
				t.Errorf("Unquote(%#q): got %v, want no error", test.input, err)
			} else {
				t.Logf("Unquote(%#q): got expected error: %v", test.input, err)
			}
		} else if err == nil && test.fail {
			t.Errorf("Unquote(%#q): got nil, want error", test.input)
		}
		if cmp := string(got); cmp != test.want {
			t.Errorf("Unquote(%#q): got %#q, want %#q", test.input, cmp, test.want)
		}
	}
}
