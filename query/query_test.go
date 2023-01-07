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

	t.Run("Const", func(t *testing.T) {
		tests := []struct {
			name  string
			query query.Query
			want  string
		}{
			{"String", query.String("foo"), `"foo"`},
			{"Float", query.Float(-3.1), `-3.1`},
			{"Integer", query.Int(17), `17`},
			{"True", query.Bool(true), `true`},
			{"False", query.Bool(false), `false`},
			{"Null", query.Null, `null`},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				v, err := query.Eval(val, test.query)
				if err != nil {
					t.Errorf("Eval failed: %v", err)
				} else if got := v.JSON(); got != test.want {
					t.Errorf("Result: got %#q, want %#q", got, test.want)
				}
			})
		}
	})

	t.Run("Seq", func(t *testing.T) {
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Path(0),
			query.Path("airDate"),
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("Key", func(t *testing.T) {
		v, err := query.Eval(val, query.Path("episodes", 0, "airDate"))
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("Alt", func(t *testing.T) {
		if v, err := query.Eval(val, query.Alt{}); err == nil {
			t.Errorf("Empty Alt: got %+v, want error", v)
		}
		v, err := query.Eval(val, query.Alt{
			query.Path(0),
			query.Path("episodes"),
			query.Null,
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if s, ok := v.(ast.Array); !ok {
			t.Errorf("Result: got %T, want array", v)
		} else if len(s) != wantLength {
			t.Errorf("Result: got %d elements, want %d", len(s), wantLength)
		}
	})

	t.Run("Slice", func(t *testing.T) {
		const wantJSON = `["2020-03-27","2020-03-26","2020-03-25"]`
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Slice(-3, 0),
			query.Each(query.Path("airDate")),
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if arr, ok := v.(ast.Array); !ok {
			t.Errorf("Result: got %T, want array", v)
		} else if got := arr.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Sub1", func(t *testing.T) {
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Sub(query.Path("guestNames", 0)),
			query.Slice(0, 3),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		a, ok := v.(ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		const wantJSON = `["Paul Rosenzweig","Mike Chase","Shane Harris"]`
		if got := a.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Sub2", func(t *testing.T) {
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Sub(query.Path("title")),
			query.Slice(0, 4),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		a, ok := v.(ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		const wantJSON = `["Chatter podcast","Book","Book","Articles"]`
		if got := a.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Sub3", func(t *testing.T) {
		v, err := query.Eval(val, query.Sub(query.Path("nonesuch")))
		if err == nil {
			t.Fatalf("Eval: got %T, wanted error", v)
		}
	})

	t.Run("Each", func(t *testing.T) {
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Each(query.Path("airDate")),
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
			t.Logf("Element %d: %v", i, elt)
		}
	})

	t.Run("Object", func(t *testing.T) {
		v, err := query.Eval(val, query.Object{
			"first":  query.Path("episodes", 0, "airDate"),
			"length": query.Seq{query.Path("episodes"), query.Len()},
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
		} else if got := first.Value.String(); got != wantString {
			t.Errorf("First: got %q, want %q", got, wantString)
		}
		if length := obj.Find("length"); length == nil {
			t.Error(`Missing "length" in result`)
		} else if got := length.Value.(ast.Int).Value(); got != wantLength {
			t.Errorf("Result: got length %d, want %d", got, wantLength)
		}
	})

	t.Run("Array", func(t *testing.T) {
		v, err := query.Eval(val, query.Array{
			query.Seq{query.Path("episodes"), query.Len()},
			query.Path("episodes", 0, "hasDetail"),
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
		if got := arr[0].(ast.Int).Value(); got != wantLength {
			t.Errorf("Entry 0: got length %d, want %d", got, wantLength)
		}
		if hasDetail := arr[1].(ast.Bool).Value(); hasDetail {
			t.Errorf("Entry 1: got hasDetail %v, want false", hasDetail)
		}
	})

	t.Run("Mixed", func(t *testing.T) {
		const wantJSON = `[18,67,56,54,52]`
		v, err := query.Eval(val, query.Seq{
			query.Path("episodes"),
			query.Slice(0, 5),
			query.Each(query.Path("summary")),
			query.Each(query.Len()),
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})
}
