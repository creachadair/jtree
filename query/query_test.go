package query_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/query"
)

func TestQuery(t *testing.T) {
	input, err := os.ReadFile("../testdata/input.json")
	if err != nil {
		t.Fatalf("Reading test input: %v", err)
	}

	vals, err := ast.Parse(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	} else if len(vals) == 0 {
		t.Fatal("Parse returned no values")
	}
	val := vals[0]

	t.Run("Seq", func(t *testing.T) {
		const wantString = "2021-11-30"

		v, err := query.Eval(val, query.Seq{
			query.Key("episodes"),
			query.Index(0),
			query.Key("airDate"),
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if s, ok := v.(*ast.String); !ok {
			t.Errorf("Result: got %T, want string", v)
		} else if got := s.Unescape(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("Each", func(t *testing.T) {
		v, err := query.Eval(val, query.Seq{
			query.Key("episodes"),
			query.Each(query.Key("airDate")),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		a, ok := v.(*ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		for i, elt := range a.Values[:5] {
			t.Logf("Element %d: %v", i, elt.(*ast.String).Unescape())
		}
	})
}
