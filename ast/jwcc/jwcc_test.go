package jwcc

import (
	"fmt"
	"strings"
	"testing"
)

func TestBasic(t *testing.T) {
	input := strings.NewReader(`// This is a JWCC document
{
  // Hello, I am an object member.
  "name": ["value"], // and a trailing comment
  "list": [
     // whatever else you may think
     // this is pretty cool
     true,
     false,
     null,
   ],  // is it me or is this stinky

   "num": /* stuff */ 12.5 /* nonsense */,

   "horse":
       "pucky" // shenanigans
  , // rumpus
   // stuff at the end
}
/*
  Various additional nonsense following the main document
  which will get bunged on after.
*/

`)
	d, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	t.Log(input)
	walk(t, d, "")
	t.Log(d.JSON())
}

func walk(t *testing.T, v Value, p string) {
	t.Logf("%s%T", p, v)
	c := v.Comments()
	if len(c.Before) != 0 {
		t.Logf("%s- before\n%s", p, strings.Join(c.Before, "\n"))
	}
	if c.Line != "" {
		t.Logf("%s- line: %s", p, c.Line)
	}
	if len(c.End) != 0 {
		t.Logf("%s- end\n%s", p, strings.Join(c.End, "\n"))
	}
	switch q := v.(type) {
	case *Array:
		for i, elt := range q.Values {
			walk(t, elt, fmt.Sprintf("  %s[%d] ", p, i))
		}
	case *Member:
		t.Logf("%s key=%q", p, q.Key)
		walk(t, q.Value, p+"  ")
	case *Object:
		for _, elt := range q.Members {
			walk(t, elt, p+"  ")
		}
	case *Datum:
		t.Logf("%s datum=%v", p, q.JSON())
	case *Document:
		walk(t, q.Value, p+"  ")
	}
}
