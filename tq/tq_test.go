package tq_test

import (
	"bytes"
	"errors"
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

	val, err := ast.ParseSingle(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	mustEval := func(t *testing.T, q tq.Query) ast.Value {
		t.Helper()
		v, err := tq.Eval(val, q)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		return v
	}

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
				v := mustEval(t, test.query)
				if got := v.JSON(); got != test.want {
					t.Errorf("Result: got %#q, want %#q", got, test.want)
				}
			})
		}
	})

	t.Run("Seq", func(t *testing.T) {
		v := mustEval(t, tq.Path("episodes", 0, "airDate"))
		if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("Key", func(t *testing.T) {
		v := mustEval(t, tq.Path("episodes", 0, "airDate"))
		if got := v.String(); got != wantString {
			t.Errorf("Result: got %q, want %q", got, wantString)
		}
	})

	t.Run("EmptyAlt", func(t *testing.T) {
		if v, err := tq.Eval(val, tq.Alt{}); err == nil {
			t.Errorf("Empty Alt: got %+v, want error", v)
		}
	})

	t.Run("Alt", func(t *testing.T) {
		v := mustEval(t, tq.Alt{
			tq.Path(0),
			tq.Path("episodes"),
			tq.Value(nil),
		})
		if s, ok := v.(ast.Array); !ok {
			t.Errorf("Result: got %T, want array", v)
		} else if len(s) != wantLength {
			t.Errorf("Result: got %d elements, want %d", len(s), wantLength)
		}
	})

	t.Run("Slice", func(t *testing.T) {
		const wantJSON = `["2020-03-27","2020-03-26","2020-03-25"]`
		v := mustEval(t, tq.Path(
			"episodes", tq.Slice(-3, 0), tq.Each("airDate"),
		))
		if arr, ok := v.(ast.Array); !ok {
			t.Errorf("Result: got %T, want array", v)
		} else if got := arr.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Recur1", func(t *testing.T) {
		v := mustEval(t, tq.Path("episodes", tq.Recur("guestNames", 0), tq.Slice(0, 3)))
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
		v := mustEval(t, tq.Seq{
			tq.Path("episodes", tq.Recur("title")),
			tq.Slice(0, 4),
		})
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
		v := mustEval(t, tq.Path("episodes", tq.Recur("url"), tq.Len()))
		const wantJSON = `183` // grep '"url"' testdata/input.json | wc -l
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Glob", func(t *testing.T) {
		// The number of fields in the first object of the episodes array.
		v := mustEval(t, tq.Path("episodes", 0, tq.Glob(), tq.Len()))
		if n, ok := v.(ast.Int); !ok {
			t.Errorf("Result: got %T, want number", v)
		} else if n != 6 {
			t.Errorf("Result: got %v, want 5", n)
		}
	})

	t.Run("RecurGlob", func(t *testing.T) {
		v := mustEval(t, tq.Seq{
			tq.Recur("links", -1), // the last link object of each set
			tq.Each(tq.Glob(), 0), // the first field of each such object
			tq.Path(-5),           // the fifth from the end
		})
		const want = "New York Times"
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %#q", got, want)
		}
	})

	t.Run("Pick", func(t *testing.T) {
		v := mustEval(t, tq.Seq{
			tq.Recur("episode"),
			tq.Pick(0, -1, 5, -3),
		})
		const wantJSON = `[557,"pilot",552,1]`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Each", func(t *testing.T) {
		v := mustEval(t, tq.Path("episodes", tq.Each("airDate"), tq.Slice(-5, 0)))
		const wantJSON = `["2020-03-29","2020-03-28","2020-03-27","2020-03-26","2020-03-25"]`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Object", func(t *testing.T) {
		v := mustEval(t, tq.Object{
			"first":  tq.Path("episodes", 0, "airDate"),
			"length": tq.Path("episodes", tq.Len()),
		})
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
		} else if got := length.Value.(ast.Int); got != wantLength {
			t.Errorf("Result: got length %d, want %d", got, wantLength)
		}
	})

	t.Run("Array", func(t *testing.T) {
		v := mustEval(t, tq.Array{
			tq.Path("episodes", tq.Len()),
			tq.Path("episodes", 0, "hasDetail"),
		})
		arr, ok := v.(ast.Array)
		if !ok {
			t.Fatalf("Result: got %T, want array", v)
		}
		if len(arr) != 2 {
			t.Fatalf("Result: got %d values, want %d", len(arr), 2)
		}
		if got := arr[0].(ast.Int); got != wantLength {
			t.Errorf("Entry 0: got length %d, want %d", got, wantLength)
		}
		if hasDetail := arr[1].(ast.Bool); hasDetail {
			t.Errorf("Entry 1: got hasDetail %v, want false", hasDetail)
		}
	})

	t.Run("Mixed", func(t *testing.T) {
		const wantJSON = `[18,67,56,54,52]`
		v := mustEval(t, tq.Seq{
			tq.Path("episodes", tq.Slice(0, 5)),
			tq.Each("summary", tq.Len()),
		})
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Select", func(t *testing.T) {
		v := mustEval(t, tq.Path(
			"episodes", tq.Exists("guestNames"), tq.Each("guestNames", 0), -1,
		))
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

		v := mustEval(t, tq.Path(
			tq.Recur("episode"),
			filter, multiply, tq.Slice(-3, 0), 0,
		))
		const want = 5533
		if got := v.(ast.Int); got != want {
			t.Errorf("Result: got %#q, want %#q", v, want)
		}
	})

	t.Run("LetGet", func(t *testing.T) {
		v := mustEval(t, tq.Let{
			// Let g be all the episode objects that define guestNames.
			{"g", tq.Path("episodes", tq.Exists("guestNames"))},
			// Let f be the third such episode.
			{"f", tq.Path("$g", 2)},
		}.In(tq.Object{
			"count":  tq.Path("$g", tq.Len()),
			"number": tq.Path("$f", "episode"),
			"name":   tq.Path(tq.Get("$f"), "guestNames", 0),
		}))
		o := v.(ast.Object)
		o.Sort()
		const wantJSON = `{"count":468,"name":"Shane Harris","number":554}`
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("Func", func(t *testing.T) {
		v := mustEval(t, tq.Seq{
			tq.Path("episodes", 1),

			// The input is the first episode.
			tq.Func(func(e tq.Env, in ast.Value) (ast.Value, error) {
				// Verify that we got the right input.
				const wantFirst = 556
				n := in.(ast.Object).Find("episode").Value.(ast.Numeric).Int()
				if n != wantFirst {
					t.Errorf("Wrong input: got %v, want %v", n, wantFirst)
				}

				// Look up the root of the original input from e.
				root := e.Get("$")
				if root == nil {
					return nil, errors.New("missing root")
				}

				// Put some stuff in the environment.
				e10, err := e.Eval(root, tq.Path("episodes", 10))
				if err != nil {
					return nil, err
				}
				e.Set("e10", e10) // this is visible to the rest of the query

				// Ignore the input and do something else.
				return ast.Bool(true), nil
			}),

			// Verify that the environment changes from the Func are visible here.
			tq.Path("$e10", "episode"),
		})

		const wantOut = 547
		if got := v.(ast.Numeric).Int(); got != wantOut {
			t.Errorf("Result: got %v, want %v", v, wantOut)
		}
	})

	t.Run("CleanFunc", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Seq{
			tq.Path("episodes", 0),
			tq.Func(func(e tq.Env, in ast.Value) (ast.Value, error) {
				// When a function evaluates in a clean environment, the changes
				// from the subquery should not be visible outside.
				return e.New().Eval(in, tq.Path("airDate", tq.Set("@")))
			}),
			tq.Get("@"), // this should fail
		})
		if err == nil {
			t.Errorf("Eval: got %#q, want error", v)
		} else {
			t.Logf("Eval: got error: %v (OK)", err)
		}
	})

	t.Run("Set", func(t *testing.T) {
		v := mustEval(t, tq.Seq{
			tq.Path(tq.Value(true), tq.Set("x")),
			tq.Alt{
				tq.Get("y"), // fails (y is not bound)

				// This query fails, so its set of x does not take effect.
				tq.Path(tq.Value(nil), tq.Set("x"), tq.Func(failq)),

				tq.Get("x"),     // succeeds (x is true)
				tq.Value(false), // not reached (previous alternative succeeded)
			},
		})

		if got, ok := v.(ast.Bool); !ok || !bool(got) {
			t.Errorf("Result: got %#q, want true", v)
		}
	})

	t.Run("Ref", func(t *testing.T) {
		v := mustEval(t, tq.Let{
			{"x", tq.Value("airDate")},
			{"p", tq.Value(25)},
		}.In(
			"episodes", tq.Ref("$p"), tq.Ref("$x"),
		))
		const want = `2021-10-19`
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %#q", got, want)
		}
	})

	t.Run("RefWild", func(t *testing.T) {
		v := mustEval(t, tq.Path(
			"episodes",

			// Use the fifth episode number from the end as the lookup index.
			tq.Ref("$", "episodes", -5, "episode"),

			"airDate",
		))
		const want = `2021-11-18`
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %#q", got, want)
		}
	})

	t.Run("KeysObj", func(t *testing.T) {
		v := mustEval(t, tq.Path("episodes", 0, tq.Keys(), -1))
		const want = "hasDetail"
		if got := v.String(); got != want {
			t.Errorf("Result: got %#q, want %q", v, want)
		}
	})

	t.Run("KeysNull", func(t *testing.T) {
		v := mustEval(t, tq.Path(ast.Null, tq.Keys()))
		const wantJSON = `[]` // empty array
		if got := v.JSON(); got != wantJSON {
			t.Errorf("Result: got %#q, want %#q", got, wantJSON)
		}
	})

	t.Run("KeysOther", func(t *testing.T) {
		v, err := tq.Eval(val, tq.Path("episodes", tq.Keys()))
		if err == nil {
			t.Errorf("Eval: got %#q, want error", v)
		} else {
			t.Logf("Eval: got error: %v (OK)", err)
		}
	})
}

func failq(tq.Env, ast.Value) (ast.Value, error) { return nil, errors.New("gratuitous failure") }
