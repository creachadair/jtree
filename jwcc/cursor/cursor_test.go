// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package cursor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/internal/testutil"
	"github.com/creachadair/jtree/jwcc"
	"github.com/creachadair/jtree/jwcc/cursor"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

//go:embed testdata/cursor.jwcc
var testJWCC string

func TestCursor(t *testing.T) {
	doc, err := jwcc.Parse(strings.NewReader(testJWCC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name string
		path []any
		want jwcc.Value
		fail bool
	}{
		{"NilInput", nil, doc.Value, false},
		{"NoMatch", []any{"nonesuch"}, doc.Value, true},
		{"WrongType", []any{11}, doc.Value, true},

		{"ArrayPos", []any{"list", 1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayNeg", []any{"list", -1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayRange", []any{"o", 25},
			doc.Value.(*jwcc.Object).Find("o").Value,
			true,
		},
		{"ObjPath", []any{"xyz", "d"},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d"),
			false,
		},
		{"ObjPathTail", []any{"xyz", "d", nil},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d").Value,
			false,
		},

		{"FuncArray", []any{"o", testPathFunc}, jwcc.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, jwcc.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d").Value,
			true,
		},
	}
	opt := cmp.AllowUnexported(
		ast.String("").Quote(),
		testutil.RawNumberType,
		jwcc.Array{},
		jwcc.Comments{},
		jwcc.Datum{},
		jwcc.Member{},
		jwcc.Object{},
	)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := cursor.New(doc.Value).Down(tc.path...)
			err := c.Err()
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Down %+v: unexpected error: %v", tc.path, err)
				}
			}
			got := c.Value()
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Down %+v: wrong result (-got, +want):\n%s", tc.path, diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
				t.Log("Path:")
				for i, p := range c.Path() {
					t.Logf("%d: %+v", i+1, p)
				}
			}
		})
	}
}

func testPathFunc(v jwcc.Value) (jwcc.Value, error) {
	switch t := v.(type) {
	case *jwcc.Array:
		return jwcc.ToValue(len(t.Values)), nil
	case *jwcc.Object:
		return jwcc.ToValue(len(t.Members)), nil
	default:
		return nil, errors.New("not a thing with length")
	}
}
