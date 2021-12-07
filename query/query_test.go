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

	const wantString = "2021-11-30"
	const wantLength = 563

	t.Run("Seq", func(t *testing.T) {
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
		a, ok := v.(ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		if len(a) != wantLength {
			t.Errorf("Result: got %d elements, want %d", len(a), wantLength)
		}
		for i, elt := range a[:5] {
			t.Logf("Element %d: %v", i, elt.(*ast.String).Unescape())
		}
	})

	t.Run("Object", func(t *testing.T) {
		v, err := query.Eval(val, query.Object{
			"first":  query.Seq{query.Key("episodes"), query.Index(0), query.Key("airDate")},
			"length": query.Seq{query.Key("episodes"), query.Len()},
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		obj, ok := v.(ast.Object)
		if !ok {
			t.Fatalf("Result: got %T, want object", v)
		}
		if first := obj.Find("first"); first == nil {
			t.Error(`Missing "first" in result`)
		} else if got := first.Value.(*ast.String).Unescape(); got != wantString {
			t.Errorf("First: got %q, want %q", got, wantString)
		}
		if length := obj.Find("length"); length == nil {
			t.Error(`Missing "length" in result`)
		} else if got := length.Value.(*ast.Integer).Int64(); got != wantLength {
			t.Errorf("Result: got length %d, want %d", got, wantLength)
		}
	})

	t.Run("Array", func(t *testing.T) {
		v, err := query.Eval(val, query.Array{
			query.Seq{query.Key("episodes"), query.Len()},
			query.Seq{query.Key("episodes"), query.Index(0), query.Key("hasDetail")},
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		arr, ok := v.(ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		if len(arr) != 2 {
			t.Fatalf("Result: got %d values, want %d", len(arr), 2)
		}
		if got := arr[0].(*ast.Integer).Int64(); got != wantLength {
			t.Errorf("Entry 0: got length %d, want %d", got, wantLength)
		}
		if hasDetail := arr[1].(*ast.Bool).Value(); hasDetail {
			t.Errorf("Entry 1: got hasDetail %v, want false", hasDetail)
		}
	})

	t.Run("Null", func(t *testing.T) {
		v, err := query.Eval(val, query.Null)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		if _, ok := v.(*ast.Null); !ok {
			t.Fatalf("Result: got %T, want null", v)
		}
	})
}
