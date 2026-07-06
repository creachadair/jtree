// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/cursor"
	"github.com/creachadair/jtree/jwcc"
	"github.com/creachadair/mds/mtest"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

var outputFile = flag.String("output", "", "Write formatted output to this file")

//go:embed testdata/basic.jwcc
var basicInput string

func TestBasic(t *testing.T) {
	var w io.Writer = os.Stdout

	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			t.Fatalf("Create output file: %v", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}()
		w = f
	}

	input := strings.NewReader(basicInput)
	d, err := jwcc.Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	u := d.Undecorate()
	djson := d.JSON()
	t.Logf("Plain JSON: %s", djson)

	ujson := u.JSON()
	if diff := cmp.Diff(djson, ujson); diff != "" {
		t.Errorf("Incorrect JSON (-want, +got):\n%s", diff)
	}

	c := cursor.New(d.Value).Down("p").Up()
	if err := c.Err(); err != nil {
		t.Errorf("Cursor: %v", err)
	}
	if pm, ok := c.Value().(*jwcc.Member); !ok {
		t.Errorf("Cursor: got %T, want member", c.Value())
	} else {
		pm.Comments().Before = []string{
			"/* All you are about to do\nin the cathedral of your soul",
			"has come true\nin your dreams.",
			"/* time to die",
		}
	}

	o := d.Value.(*jwcc.Object)
	m := jwcc.Field("large", "snakes")
	m.Comments().Before = []string{"Listen, I have had it up to here\nwith all these"}
	m.Comments().Line = "/*in this aircraft"
	o.Members = append(o.Members, m)

	if err := jwcc.Format(w, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
}

func TestOneValueOnly(t *testing.T) {
	// This input contains a document with some comments before and after, which
	// is fine, but then a second value which is not.
	const input = `// before
{"x": "y", // ok
} // also ok
["bogus"]`

	d, err := jwcc.Parse(strings.NewReader(input))
	if !errors.Is(err, ast.ErrExtraInput) {
		t.Errorf("Got %#v, %v; want %v", d, err, ast.ErrExtraInput)
	}
}

func TestArrayOf(t *testing.T) {
	indent := func(v jwcc.Value) string { return jwcc.FormatToString(v) }
	t.Run("Empty", func(t *testing.T) {
		if diff := cmp.Diff(indent(jwcc.ArrayOf[any]()), `[]`); diff != "" {
			t.Errorf("ArrayOf() (-got, +want):\n%s", diff)
		}
	})
	t.Run("Strings", func(t *testing.T) {
		if diff := cmp.Diff(indent(jwcc.ArrayOf("a", "b", "c")), `["a", "b", "c"]`); diff != "" {
			t.Errorf("ArrayOf strings (-got, +want):\n%s", diff)
		}
	})
	t.Run("Single", func(t *testing.T) {
		if diff := cmp.Diff(indent(jwcc.ArrayOf[any](&jwcc.Object{
			Members: []*jwcc.Member{jwcc.Field("alpha", true)},
		})), `[{"alpha": true}]`); diff != "" {
			t.Errorf("ArrayOf simple object (-got, +want):\n%s", diff)
		}
	})
	t.Run("Mixed", func(t *testing.T) {
		got := jwcc.ArrayOf[any](&jwcc.Object{
			Members: []*jwcc.Member{jwcc.Field("foo", "bar")},
		}, "baz", 123, false)
		if diff := cmp.Diff(indent(got), `[
  {"foo": "bar"},
  "baz",
  123,
  false,
]`); diff != "" {
			t.Errorf("ArrayOf mixed (-got, +want):\n%s", diff)
		}
	})
}

func TestFormatOptions(t *testing.T) {
	setComment := func(v jwcc.Value, text string) jwcc.Value {
		v.Comments().Line = text
		return v
	}
	tests := []struct {
		name  string
		input jwcc.Value
		opts  jwcc.Formatter
		want  string
	}{
		{
			name:  "default_array_3",
			input: jwcc.ArrayOf("a", "b", "c"),
			opts:  jwcc.Formatter{},
			want:  `["a", "b", "c"]`,
		},
		{
			name:  "default_array_4",
			input: jwcc.ArrayOf(1, 2, 3, 4),
			opts:  jwcc.Formatter{},
			want:  "[\n  1,\n  2,\n  3,\n  4,\n]",
		},
		{
			name:  "count_4_array_3",
			input: jwcc.ArrayOf("a", "b", "c"),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 4},
			want:  `["a", "b", "c"]`,
		},
		{
			name:  "count_2_array_3",
			input: jwcc.ArrayOf(1, 2, 3),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 2},
			want:  "[\n  1,\n  2,\n  3,\n]",
		},
		{
			name:  "count_0_length_10_short",
			input: jwcc.ArrayOf("a", "b"),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 10},
			want:  `["a", "b"]`,
		},
		{
			name:  "count_0_length_10_commented",
			input: setComment(jwcc.ArrayOf(25, 37), "Something to say"),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 10},
			want:  "[\n  25,\n  37,\n]   // Something to say\n",
		},
		{
			name:  "count_0_length_50_commented",
			input: setComment(jwcc.ArrayOf(25, 37), "Something to say"),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 50},
			want:  "[25, 37] // Something to say\n",
		},
		{
			name:  "count_0_length_10_long",
			input: jwcc.ArrayOf(12345, 67890, 23456),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 10},
			want:  "[\n  12345,\n  67890,\n  23456,\n]",
		},
		{
			name:  "count_0_length_90_array_5",
			input: jwcc.ArrayOf("alpha", "bravo", "charlie", "delta", "echo", "foxtrot"),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 90},
			want:  `["alpha", "bravo", "charlie", "delta", "echo", "foxtrot"]`,
		},
		{
			name:  "count_4_length_90_array_5",
			input: jwcc.ArrayOf(1, 2, 3, 4, 5),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 4, MaxInlineArrayLength: 90},
			want:  "[\n  1,\n  2,\n  3,\n  4,\n  5,\n]",
		},
		{
			name:  "Nested/count_4_array_3",
			input: jwcc.ObjectOf(jwcc.Field("foo", jwcc.ArrayOf("a", "b", "c"))),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 4},
			want:  `{"foo": ["a", "b", "c"]}`,
		},
		{
			name:  "Nested/count_2_array_3",
			input: jwcc.ObjectOf(jwcc.Field("foo", jwcc.ArrayOf(1, 2, 3))),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 2},
			want:  "{\n  \"foo\": [\n    1,\n    2,\n    3,\n  ],\n}",
		},
		{
			name:  "Nested/count_0_length_10_short",
			input: jwcc.ObjectOf(jwcc.Field("foo", jwcc.ArrayOf("a", "b"))),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 10},
			want:  `{"foo": ["a", "b"]}`,
		},
		{
			name:  "Nested/count_0_length_10_commented",
			input: jwcc.ObjectOf(jwcc.Field("foo", setComment(jwcc.ArrayOf(23, 37), "history and tradition"))),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 10},
			want:  "{\n  \"foo\": [\n    23,\n    37,\n  ], // history and tradition\n}",
		},
		{
			name:  "Nested/count_0_length_50_commented",
			input: jwcc.ObjectOf(jwcc.Field("foo", setComment(jwcc.ArrayOf(25, 37), "history and tradition"))),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 50},
			want:  "{\n  \"foo\": [25, 37], // history and tradition\n}",
		},
		{
			name:  "Nested/count_0_length_90_array_5",
			input: jwcc.ObjectOf(jwcc.Field("foo", jwcc.ArrayOf("apple", "pear", "plum", "cherry", "quince"))),
			opts:  jwcc.Formatter{MaxInlineArrayLength: 90},
			want:  `{"foo": ["apple", "pear", "plum", "cherry", "quince"]}`,
		},
		{
			name: "Nested/count_0_length_90_array_5_commented",
			input: jwcc.ObjectOf(jwcc.Field("foo", setComment(
				jwcc.ArrayOf(123, 456, 789, 1011, 1213), "no laws just vibes"))),
			opts: jwcc.Formatter{MaxInlineArrayLength: 90},
			want: "{\n  \"foo\": [123, 456, 789, 1011, 1213], // no laws just vibes\n}",
		},
		{
			name: "Nested/count_0_length_20_array_5_commented",
			input: jwcc.ObjectOf(jwcc.Field("foo", setComment(
				jwcc.ArrayOf(123, 456, 789, 1011, 1213), "no laws just vibes"))),
			opts: jwcc.Formatter{MaxInlineArrayLength: 20},
			want: "{\n  \"foo\": [\n    123,\n    456,\n    789,\n    1011,\n    1213,\n  ], // no laws just vibes\n}",
		},
		{
			name:  "Nested/count_4_length_90_array_5",
			input: jwcc.ObjectOf(jwcc.Field("foo", jwcc.ArrayOf(1, 2, 3, 4, 5))),
			opts:  jwcc.Formatter{MaxInlineArrayElements: 4, MaxInlineArrayLength: 90},
			want:  "{\n  \"foo\": [\n    1,\n    2,\n    3,\n    4,\n    5,\n  ],\n}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.opts.Format(&buf, tc.input); err != nil {
				t.Fatalf("Format input %+v: unexpected error: %v", tc.input, err)
			}
			if diff := cmp.Diff(buf.String(), tc.want); diff != "" {
				t.Errorf("Format %+v (-got, +want):\n%s", tc.input, diff)
			}
		})
	}
}

func TestCleanComments(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{nil, nil},
		{[]string{}, nil},

		{[]string{"hocus pocus"}, []string{"hocus pocus"}},

		{[]string{
			"// a fine mess\nyou have gotten me into", // note multiple lines
			"// here",
			"today",
		}, []string{
			"a fine mess",
			"you have gotten me into",
			"here",
			"today",
		}},

		{[]string{
			"/* I knew you were\ntrouble when you\nwalked in */",
		}, []string{
			"I knew you were",
			"trouble when you",
			"walked in",
		}},

		{[]string{
			"plain text",
			"// line comment",
			"// another line\nand more",
			"/*\n  also a block comment\n  that I found\n  \n*/\n",
			"more plain text",
		}, []string{
			"plain text",
			"line comment",
			"another line",
			"and more",
			"also a block comment",
			"that I found",
			"more plain text",
		}},
	}
	for _, tc := range tests {
		got := jwcc.CleanComments(tc.input...)
		if diff := cmp.Diff(got, tc.want); diff != "" {
			t.Errorf("CleanComments %+q (-got, +want):%s", tc.input, diff)
		}
	}
}

func TestDecorate(t *testing.T) {
	t.Run("Plain", func(t *testing.T) {
		s := jwcc.ToValue("bar")
		s.Comments().Line = "already commented"
		in := ast.Array{
			ast.ToValue("foo"),
			ast.ToValue(1),
			ast.ToValue(true),
			ast.ToValue(nil),
			s,
		}
		out, ok := jwcc.Decorate(in).(*jwcc.Array)
		if !ok {
			t.Fatalf("Incorrect type: %T", out)
		}
		if got, want := out.JSON(), in.JSON(); got != want {
			t.Errorf("Decorated JSON: got %q, want %q", got, want)
		}
		for i, v := range out.Values {
			if c := v.Comments(); c.Line == "" {
				c.Line = fmt.Sprintf("comment %d", i+1)
			}
		}
		t.Logf("Result:\n%s", jwcc.FormatToString(out))
	})

	t.Run("Decorated", func(t *testing.T) {
		in := jwcc.ToValue("foo")
		in.Comments().Line = "hello world"

		// A value that is already decorated must not be decorated further.
		out := jwcc.Decorate(in)
		if out != in {
			t.Errorf("Decorate JWCC: got %[1]T (%[1]v), want %[2]T (%[2]v)", out, in)
		}

		got := jwcc.FormatToString(out)
		want := jwcc.FormatToString(in)
		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("Decorated JWCC (-got, +want):\n%s", diff)
		}
	})
}

func TestToValue(t *testing.T) {
	t.Run("Null", func(t *testing.T) {
		got := jwcc.ToValue(nil)
		if d, ok := got.(*jwcc.Datum); !ok || d.Value != ast.Null {
			t.Errorf("got %[1]T %[1]v, want datum null", got)
		}
	})
	t.Run("String", func(t *testing.T) {
		got := jwcc.ToValue("fuzzy")
		if d, ok := got.(*jwcc.Datum); !ok || d.Value.String() != "fuzzy" {
			t.Errorf("got %[1]T %[1]v, want string fuzzy", got)
		}
	})
	t.Run("True", func(t *testing.T) {
		got := jwcc.ToValue(true)
		if d, ok := got.(*jwcc.Datum); !ok || d.Value.String() != "true" {
			t.Errorf("got %[1]T %[1]v, want bool true", got)
		}
	})
	t.Run("Array", func(t *testing.T) {
		got := jwcc.ToValue(ast.ArrayOf(1, 2, 3))
		if a, ok := got.(*jwcc.Array); !ok || a.JSON() != `[1,2,3]` {
			t.Errorf("got %[1]T %[1]v, want array [1,2,3]", got)
		}
	})
	t.Run("Object", func(t *testing.T) {
		got := jwcc.ToValue(ast.Object{
			ast.Field("foo", 1),
			ast.Field("bar", true),
		})
		if o, ok := got.(*jwcc.Object); !ok || o.JSON() != `{"foo":1,"bar":true}` {
			t.Errorf("got %[1]T %[1]v, want object", got)
		}
	})
	t.Run("Invalid", func(t *testing.T) {
		mtest.MustPanic(t, func() { jwcc.ToValue([]bool{true}) })
		mtest.MustPanic(t, func() { jwcc.ToValue(func() {}) })
		mtest.MustPanic(t, func() { jwcc.ToValue(make(chan struct{})) })
	})
}
