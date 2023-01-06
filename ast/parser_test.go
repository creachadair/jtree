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

	root, ok := vs[0].(ast.Object)
	if !ok {
		t.Fatalf("Root is %T, not object", vs[0])
	}
	mem := root.Find("episodes")
	if mem == nil {
		t.Fatal(`Key "episodes" not found`)
	}
	lst, ok := mem.Value.(ast.Array)
	if !ok {
		t.Fatalf("Member value is %T, not array", mem.Value)
	} else if len(lst) == 0 {
		t.Fatal("Array value is empty")
	}
	obj, ok := lst[0].(ast.Object)
	if !ok {
		t.Fatalf("Array entry is %T, not object", lst[0])
	}

	ep := obj.Find("summary")
	if ep == nil {
		t.Fatal(`Key "summary" not found`)
	}

	str, ok := ep.Value.(*ast.String)
	if !ok {
		t.Fatalf("Member value is %T, not string", ep.Value)
	}
	t.Logf("String field value: %s", str.Unescape())
}

func TestString(t *testing.T) {
	tests := []struct {
		input ast.Value
		want  string
	}{
		{&ast.Null{}, "null"},

		{ast.NewBool(false), "false"},
		{ast.NewBool(true), "true"},

		{ast.NewString(""), `""`},
		{ast.NewString("a \t b"), `"a \t b"`},

		{ast.NewNumber(-0.00239), `-0.00239`},

		{ast.NewInteger(0), `0`},
		{ast.NewInteger(15), `15`},
		{ast.NewInteger(-25), `-25`},

		{ast.Array{}, `[]`},
		{ast.Array{
			ast.NewBool(false),
		}, `[false]`},
		{ast.Array{
			ast.NewBool(true),
			ast.NewInteger(199),
		}, `[true,199]`},
		{ast.Array{
			ast.NewString("free"),
			ast.NewString("your"),
			ast.NewString("mind"),
		}, `["free","your","mind"]`},

		{ast.Object{}, `{}`},
		{ast.Object{
			ast.Field("xs", &ast.Null{}),
		}, `{"xs":null}`},
		{ast.Object{
			ast.Field("name", ast.NewString("Dennis")),
			ast.Field("age", ast.NewInteger(37)),
			ast.Field("isOld", ast.NewBool(false)),
		}, `{"name":"Dennis","age":37,"isOld":false}`},

		{ast.Object{
			ast.Field("values", ast.Array{
				ast.NewInteger(5),
				ast.NewNumber(10),
				ast.NewBool(true),
			}),
			ast.Field("page", ast.Object{
				ast.Field("token", ast.NewString("xyz-pdq-zvm")),
				ast.Field("count", ast.NewInteger(100)),
			}),
		}, `{"values":[5,10,true],"page":{"token":"xyz-pdq-zvm","count":100}}`},
	}
	for _, test := range tests {
		got := test.input.String()
		if got != test.want {
			t.Errorf("Input: %+v\nGot:  %s\nWant: %s", test.input, got, test.want)
		}
	}
}
