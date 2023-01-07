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
	obj, ok := lst[1].(ast.Object)
	if !ok {
		t.Fatalf("Array entry is %T, not object", lst[0])
	}
	check[ast.Quoted](t, obj, "summary", func(s ast.Quoted) {
		t.Logf("String field value: %s", s.Unquote())
	})
	check[ast.Number](t, obj, "episode", func(v ast.Number) {
		t.Logf("Number field value: %v", v)
		if !v.IsInt() {
			t.Errorf("Number %s should be recognized as integer", v.JSON())
		}
	})
	check[ast.Bool](t, obj, "hasDetail", func(v ast.Bool) {
		t.Logf("Bool field value: %v", v)
	})
}

func check[T any](t *testing.T, obj ast.Object, key string, f func(T)) {
	t.Helper()
	if v := obj.Find(key); v == nil {
		t.Fatalf("Key %q not found", key)
	} else if tv, ok := v.Value.(T); !ok {
		var zero T
		t.Fatalf("Key %q value is %T, not %T", key, v, zero)
	} else if f != nil {
		f(tv)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		input ast.Value
		want  string
	}{
		{ast.Null, "null"},

		{ast.Bool(false), "false"},
		{ast.Bool(true), "true"},

		{ast.String(""), `""`},
		{ast.String("a \t b"), `"a \t b"`},

		{ast.Float(-0.00239), `-0.00239`},

		{ast.Int(0), `0`},
		{ast.Int(15), `15`},
		{ast.Int(-25), `-25`},

		{ast.Array{}, `[]`},
		{ast.Array{
			ast.Bool(false),
		}, `[false]`},
		{ast.Array{
			ast.Bool(true),
			ast.Int(199),
		}, `[true,199]`},
		{ast.Array{
			ast.String("free"),
			ast.String("your"),
			ast.String("mind"),
		}, `["free","your","mind"]`},

		{ast.Object{}, `{}`},
		{ast.Object{
			ast.Field("xs", ast.Null),
		}, `{"xs":null}`},
		{ast.Object{
			ast.Field("name", ast.String("Dennis")),
			ast.Field("age", ast.Int(37)),
			ast.Field("isOld", ast.Bool(false)),
		}, `{"name":"Dennis","age":37,"isOld":false}`},

		{ast.Object{
			ast.Field("values", ast.Array{
				ast.Int(5),
				ast.Int(10),
				ast.Bool(true),
			}),
			ast.Field("page", ast.Object{
				ast.Field("token", ast.String("xyz-pdq-zvm")),
				ast.Field("count", ast.Int(100)),
			}),
		}, `{"values":[5,10,true],"page":{"token":"xyz-pdq-zvm","count":100}}`},
	}
	for _, test := range tests {
		got := test.input.JSON()
		if got != test.want {
			t.Errorf("Input: %+v\nGot:  %s\nWant: %s", test.input, got, test.want)
		}
	}
}
