// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc_test

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/jwcc"
	"github.com/creachadair/jtree/jwcc/cursor"
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

	p, err := cursor.Path(d.Value, "p")
	if err != nil {
		t.Errorf("Path: %v", err)
	} else {
		p.Comments().Before = []string{
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
