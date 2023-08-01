package jwcc_test

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/jwcc"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

var outputFile = flag.String("output", "", "Write formatted output to this file")

//go:embed testdata/input.jwcc
var testJWCC string

const basicInput = `

// This is a JWCC document
// everyone loves those

{  // Hello, I am an object member.
  "name": ["value",
"village",
], // and a trailing comment
 "list": [     // whatever else you may think
     // this is pretty cool
    true, // a
  false, // b
   null, // c
 {"pea":"brain", /*fool*/
},
  "soup" // nuts
 ],  // is it me or is this stinky
// hey all
 "num":
 /* stuff */ 12.5 /* nonsense */,
"p":"q",

   "f": {"zuul":true,
   }, // the cat
   //"x": 3,

   "horse":
       "pucky" // shenanigans
  , // rumpus
   // stuff at the end
}/* Various additional nonsense following the main document
  which will get bunged on after.*/




`

func TestBasic(t *testing.T) {
	var w io.Writer = os.Stdout

	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			t.Fatalf("Create output file: %v", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}()
		w = f
	}

	input := strings.NewReader(basicInput)
	d, err := jwcc.Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	u := d.Undecorate()
	djson := d.JSON()
	t.Logf("Plain JSON: %s", djson)

	ujson := u.JSON()
	if diff := cmp.Diff(djson, ujson); diff != "" {
		t.Errorf("Incorrect JSON (-want, +got):\n%s", diff)
	}

	if err := jwcc.Format(w, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
}

func TestPath(t *testing.T) {
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
		{"ArrayRange", []any{"o", 25}, doc.Value, true},
		{"ObjPath", []any{"xyz", "d"},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d").Value,
			false,
		},
	}
	opt := cmp.AllowUnexported(
		ast.Quoted{},
		ast.Number{},
		jwcc.Array{},
		jwcc.Comments{},
		jwcc.Datum{},
		jwcc.Member{},
		jwcc.Object{},
	)
	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			got, err := jwcc.Path(doc.Value, tc.path...)
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Path: unexpected error: %v", err)
				}
			}
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Wrong result (-got, +want):\n%s", diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
			}
		})
	}
}
