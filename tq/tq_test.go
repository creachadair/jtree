package tq_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/tq"
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

	t.Run("Value", func(t *testing.T) {
		tests := []struct {
			name  string
			query tq.Query
			want  string
		}{
			{"String", tq.Value("foo"), `"foo"`},
			{"Quoted", tq.Value(ast.String("bar").Quote()), `"bar"`},
			{"Float", tq.Value(-3.1), `-3.1`},
			{"Integer", tq.Value(17), `17`},
			{"True", tq.Value(true), `true`},
			{"False", tq.Value(false), `false`},
			{"Null", tq.Value(nil), `null`},
			{"Obj", tq.Value(ast.Object{
				ast.Field("ok", ast.Bool(true)),
			}), `{"ok":true}`},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				v, err := tq.Eval(val, test.query)
				if err != nil {
					t.Errorf("Eval failed: %v", err)
				} else if got := v.JSON(); got != test.want {
					t.Errorf("Result: got %#q, want %#q", got, test.want)
				}
			})
		}
	})

	t.Run("Seq", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Path("episodes", 0, "airDate"))
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("Key", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Path("episodes", 0, "airDate"))
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("EmptyAlt", func(t *testing.T) {
		if v, err := tq.Eval(val, tq.Alt{}); err == nil {
			t.Errorf("Empty Alt: got %+v, want error", v)
		}
	})

	t.Run("Alt", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Alt{
			tq.Path(0),
			tq.Path("episodes"),
			tq.Value(nil),
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
		v, err := tq.Eval(val, tq.Path(
			"episodes", tq.Slice(-3, 0), tq.Each("airDate"),
		))
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if arr, ok := v.(ast.Array); !ok {
			t.Errorf("Result: got %T, want array", v)
		} else if got := arr.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Recur1", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Path(
			"episodes", tq.Recur("guestNames", 0), tq.Slice(0, 3),
		))
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

	t.Run("Recur2", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Path("episodes", tq.Recur("title")),
			tq.Slice(0, 4),
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

	t.Run("Recur3", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Recur("nonesuch"))
		if err == nil {
			t.Fatalf("Eval: got %T, wanted error", v)
		}
	})

	t.Run("Count", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Path("episodes", tq.Recur("url")),
			tq.Len(),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const wantJSON = `183` // grep '"url"' testdata/input.json | wc -l
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Glob", func(t *testing.T) {
		// The number of fields in the first object of the episodes array.
		v, err := tq.Eval(val, tq.Path("episodes", 0, tq.Glob(), tq.Len()))
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		if n, ok := v.(ast.Int); !ok {
			t.Errorf("Result: got %T, want number", v)
		} else if n != 6 {
			t.Errorf("Result: got %v, want 5", n)
		}
	})

	t.Run("RecurGlob", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Recur("links", -1), // the last link object of each set
			tq.Each(tq.Glob(), 0), // the first field of each such object
			tq.Path(-5),           // the fifth from the end
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const want = "New York Times"
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %#q", got, want)
		}
	})

	t.Run("Pick", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Recur("episode"),
			tq.Pick(0, -1, 5, -3),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const wantJSON = `[557,"pilot",552,1]`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Each", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Path("episodes", tq.Each("airDate")),
			tq.Slice(-5, 0),
		})
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const wantJSON = `["2020-03-29","2020-03-28","2020-03-27","2020-03-26","2020-03-25"]`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Object", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Object{
			"first":  tq.Path("episodes", 0, "airDate"),
			"length": tq.Path("episodes", tq.Len()),
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
		v, err := tq.Eval(val, tq.Array{
			tq.Path("episodes", tq.Len()),
			tq.Path("episodes", 0, "hasDetail"),
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
		v, err := tq.Eval(val, tq.Seq{
			tq.Path("episodes", tq.Slice(0, 5)),
			tq.Each("summary", tq.Len()),
		})
		if err != nil {
			t.Errorf("Eval failed: %v", err)
		} else if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Select", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Path(
			"episodes", tq.Exists("guestNames"), tq.Each("guestNames", 0), -1,
		))
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const want = "Danielle Citron"
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %#q", got, want)
		}
	})

	t.Run("Mapping", func(t *testing.T) {
		// Choose numeric values greater than 500.
		filter := tq.Select(func(z ast.Numeric) bool { return z.Int() > 500 })

		// Multiply numeric values by 11.
		multiply := tq.Map(func(z ast.Numeric) ast.Int { return z.Int() * 11 })

		v, err := tq.Eval(val, tq.Path(
			tq.Recur("episode"),
			filter, multiply, tq.Slice(-3, 0), 0,
		))
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		const want = 5533
		if got := v.(ast.Int); got != want {
			t.Errorf("Result: got %#q, want %#q", v, want)
		}
	})
}
