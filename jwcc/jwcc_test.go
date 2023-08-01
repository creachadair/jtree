package jwcc_test

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/creachadair/jtree/jwcc"
)

var outputFile = flag.String("output", "", "Write formatted output to this file")

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

	if err := jwcc.Format(w, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
}
