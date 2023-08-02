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
	"github.com/creachadair/jtree/jwcc"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

var outputFile = flag.String("output", "", "Write formatted output to this file")

//go:embed testdata/input.jwcc
var testJWCC string

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

	p, err := jwcc.Path(d.Value, "p")
	if err != nil {
		t.Errorf("Path: %v", err)
	} else {
		p.Comments().Before = []string{
			"/* All you are about to do\nin the cathedral of your soul",
			"has come true\nin your dreams.",
			"/* time to die",
		}
	}

	if err := jwcc.Format(w, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
}

func TestPath(t *testing.T) {
	doc, err := jwcc.Parse(strings.NewReader(testJWCC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name string
		path []any
		want jwcc.Value
		fail bool
	}{
		{"NilInput", nil, doc.Value, false},
		{"NoMatch", []any{"nonesuch"}, doc.Value, true},
		{"WrongType", []any{11}, doc.Value, true},

		{"ArrayPos", []any{"list", 1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayNeg", []any{"list", -1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayRange", []any{"o", 25}, doc.Value, true},
		{"ObjPath", []any{"xyz", "d"},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d"),
			false,
		},

		{"FuncArray", []any{"o", testPathFunc}, jwcc.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, jwcc.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc}, doc.Value, true},
	}
	opt := cmp.AllowUnexported(
		ast.Quoted{},
		ast.Number{},
		jwcc.Array{},
		jwcc.Comments{},
		jwcc.Datum{},
		jwcc.Member{},
		jwcc.Object{},
	)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jwcc.Path(doc.Value, tc.path...)
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Path: unexpected error: %v", err)
				}
			}
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Wrong result (-got, +want):\n%s", diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
			}
		})
	}
}

func testPathFunc(v jwcc.Value) (jwcc.Value, error) {
	switch t := v.(type) {
	case *jwcc.Array:
		return jwcc.ToValue(len(t.Values)), nil
	case *jwcc.Object:
		return jwcc.ToValue(len(t.Members)), nil
	default:
		return nil, errors.New("not a thing with length")
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
	in := ast.Array{
		ast.ToValue("foo"),
		ast.ToValue(1),
		ast.ToValue(true),
		ast.ToValue(nil),
	}
	out, ok := jwcc.Decorate(in).(*jwcc.Array)
	if !ok {
		t.Fatalf("Incorrect type: %T", out)
	}
	if got, want := out.JSON(), in.JSON(); got != want {
		t.Errorf("Decorated JSON: got %q, want %q", got, want)
	}
	for i, v := range out.Values {
		v.Comments().Line = fmt.Sprintf("comment %d", i+1)
	}
	var buf bytes.Buffer
	jwcc.Format(&buf, out)
	t.Logf("Result:\n%s", buf.String())
}
