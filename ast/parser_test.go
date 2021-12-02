// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/creachadair/jtree/ast"
)

func TestParse(t *testing.T) {
	input, err := os.ReadFile("../testdata/input.json")
	if err != nil {
		t.Fatalf("Reading test input: %v", err)
	}

	start := time.Now()
	vs, err := ast.Parse(bytes.NewReader(input))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	t.Logf("Parsed %d bytes into %d values [%v elapsed]",
		len(input), len(vs), elapsed)
	if len(vs) == 0 {
		t.Fatal("No objects found")
	}

	// Inspect some of the structure of the test value to make sure we got
	// something approximating sense.
	//
	// If the testdata file changes, this may need to be updated.
	//
	// {
	//   "episodes": [
	//     {
	//       ...
	//       "summary": "whatever blah blah",
	//       ...
	//     },
	//     ...
	//   ]
	// }
	//

	root, ok := vs[0].(*ast.Object)
	if !ok {
		t.Fatalf("Root is %T, not object", vs[0])
	}
	mem := root.Find("episodes")
	if mem == nil {
		t.Fatal(`Key "episodes" not found`)
	}
	lst, ok := mem.Value.(*ast.Array)
	if !ok {
		t.Fatalf("Member value is %T, not array", mem.Value)
	} else if len(lst.Values) == 0 {
		t.Fatal("Array value is empty")
	}
	obj, ok := lst.Values[0].(*ast.Object)
	if !ok {
		t.Fatalf("Array entry is %T, not object", lst.Values[0])
	}

	ep := obj.Find("summary")
	if ep == nil {
		t.Fatal(`Key "summary" not found`)
	}
	span := ep.Span()
	t.Logf("Member source:\n%s", string(input[span.Pos:span.End]))

	str, ok := ep.Value.(*ast.String)
	if !ok {
		t.Fatalf("Member value is %T, not string", ep.Value)
	}
	t.Logf("String field value: %s", str.Unescape())
}
