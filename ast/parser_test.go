// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/creachadair/jtree/ast"
	"github.com/google/go-cmp/cmp"
)

var (
	_ ast.Number = ast.Int(0)
	_ ast.Number = ast.Float(0)
)

func TestParse_JWCC(t *testing.T) {
	input, err := os.ReadFile("../testdata/input.jwcc")
	if err != nil {
		t.Fatalf("Reading test input: %v", err)
	}

	p := ast.NewParser(bytes.NewReader(input))
	p.AllowJWCC(true)

	v, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// TODO(creachadair): Add tests for comment preservation, once the AST
	// supports it.
	t.Log(v.JSON())
}

func TestParse(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		vs, err := ast.Parse(strings.NewReader(""))
		if !errors.Is(err, ast.ErrEmptyInput) {
			t.Errorf("Parse empty: got %v, want %v", err, ast.ErrEmptyInput)
		}
		if len(vs) != 0 {
			t.Errorf("Parse empty: unexpected values: %v", vs)
		}
	})

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
	check[ast.Text](t, obj, "summary", func(s ast.Text) {
		t.Logf("String field value: %s", s.String())
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

func TestParseRange(t *testing.T) {
	const input = `1"foo"{ }true null[ "a" , "b" ]{"key": "value"}"stop"1 3 4`

	var got []string
	want := []string{"1", `"foo"`, "{}", "true", "null", `["a","b"]`, `{"key":"value"}`}

	for v, err := range ast.ParseRange(strings.NewReader(input)) {
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if v.String() == "stop" {
			break
		}
		got = append(got, v.JSON())
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Range elements (-got, +want):\n%s", diff)
	}
}

func TestRegression(t *testing.T) {
	// Regression: Plain values were not correctly reduced at the top level.
	t.Run("TopLevelValue", func(t *testing.T) {
		vs, err := ast.Parse(strings.NewReader(`{"p" : null}"a" 5 true [1, {}]`))
		if err != nil {
			t.Fatalf("Parse: unexpected error: %v", err)
		}
		wantJSON := []string{`{"p":null}`, `"a"`, `5`, `true`, `[1,{}]`}
		var got []string
		for _, v := range vs {
			got = append(got, v.JSON())
		}
		if diff := cmp.Diff(wantJSON, got); diff != "" {
			t.Errorf("Parse (-want, +got):\n%s", diff)
		}
	})
}

func TestParseSingle(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		for _, input := range []string{"", " ", "\n\t\n"} {
			v, err := ast.ParseSingle(strings.NewReader(input))
			if !errors.Is(err, ast.ErrEmptyInput) {
				t.Errorf("ParseSingle(%q): got error %v, want %v", input, err, ast.ErrEmptyInput)
			}
			if v != nil {
				t.Errorf("ParseSingle(%q): got result %v, want nil", input, v)
			}
		}
	})

	t.Run("Good", func(t *testing.T) {
		const input = ` [ 1, 2, 3 ]  `
		v, err := ast.ParseSingle(strings.NewReader(input))
		if err != nil {
			t.Errorf("ParseSingle: unexpected error: %v", err)
		}
		const wantJSON = `[1,2,3]`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Bad", func(t *testing.T) {
		const input = ` {"a" : 1 } {"b: 2} 3`
		v, err := ast.ParseSingle(strings.NewReader(input))
		if !errors.Is(err, ast.ErrExtraInput) {
			t.Errorf("ParseSingle: got err=%v, want %v", err, ast.ErrExtraInput)
		}
		const wantJSON = `{"a":1}`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Broken", func(t *testing.T) {
		const input = `{"x"`
		v, err := ast.ParseSingle(strings.NewReader(input))
		if err == nil {
			t.Errorf("ParseSingle: got %+q, want error", v.JSON())
		} else if errors.Is(err, ast.ErrExtraInput) {
			t.Errorf("ParseSingle: error should not be %v", err)
		}
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
			ast.Field("xs", nil),
		}, `{"xs":null}`},
		{ast.Object{
			ast.Field("name", "Dennis"),
			ast.Field("age", 37),
			ast.Field("isOld", false),
		}, `{"name":"Dennis","age":37,"isOld":false}`},

		{ast.Object{
			ast.Field("values", ast.Array{
				ast.Int(5),
				ast.Int(10),
				ast.Bool(true),
			}),
			ast.Field("page", ast.Object{
				ast.Field("token", "xyz-pdq-zvm"),
				ast.Field("count", 100),
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

func TestSpelling(t *testing.T) {
	tests := []struct {
		input ast.Text
		want  string
	}{
		{ast.String(""), ""},
		{ast.String("abc"), "abc"},
		{ast.String("").Quote(), `""`},
		{ast.Quoted(`""`).Unquote(), ""},
		{ast.String("abc").Quote(), `"abc"`},
		{ast.String("a\tb"), "a\tb"},
		{ast.String("a\tb").Quote(), `"a\tb"`},
		{ast.Quoted(`"a\tb"`).Unquote(), "a\tb"},
	}

	for _, tc := range tests {
		if got := tc.input.Spelling(); got != tc.want {
			t.Errorf("Spelling(%s): got %q, want %q", tc.input.JSON(), got, tc.want)
		}
	}
}
