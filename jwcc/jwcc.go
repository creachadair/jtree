// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

// Package jwcc implements a parser for JSON With Commas and Comments (JWCC) as
// defined by https://nigeltao.github.io/blog/2021/json-with-commas-comments.html
package jwcc

import (
	"errors"
	"io"
	"strings"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/ast"
	"golang.org/x/exp/slices"
)

// A Value is a JSON value optionally decorated with comments.
// The concrete type is one of [*Array], [*Datum], [*Document], or [*Object].
type Value interface {
	ast.Value

	// Comments returns the comments annotating this value.
	Comments() *Comments

	// Convert this value to an undecorated ast.Value.
	Undecorate() ast.Value
}

// Comments records the comments associated with a value.
// All values have a comment record; use IsEmpty to test whether the value has
// any actual comment text.
//
// Comments read during parsing are stored as-seen in the input, including
// comment markers. Line comments also include their trailing newline.  Blank
// lines separating groups of comments are represented in the Before and End
// fields as empty strings. For example, given input:
//
//	// a
//	// b
//
//	// c
//
//	"value"
//
// the resulting Before comment on "value" would be:
//
//	[]string{"// a\n", "// b\n", "", "// c\n", ""}
//
// The trailing "" indicates that the last comment did not immediately precede
// the value in the source (though the amount of separation is not stored).
//
// When adding or editing a comment programmatically, comment
// markers are optional; the text will be decorated when formatting the output.
// To include blank space separating multiple chunks of comment text, include
// an empty string. To include a blank line in the middle of a chunk, include
// a comment marker.
//
// For example:
//
//	c := jtree.Comments{Before: []string{"a", "", "b"}}
//
// renders as:
//
//	// a
//
//	// b
//
// By contrast:
//
//	c := jtree.Comments{Before: []string{"a", "//", "c"}}
//
// renders as:
//
//	// a
//	//
//	// b
//
// If a comment begins with "//", it will be processed as line comments.
// If a comment begins with "/*" it will be processed as a block comment.
// Multiple lines are OK; they will be reformatted as necessary.
type Comments struct {
	Before []string
	Line   string
	End    []string

	vloc jtree.Location // the location of the value this is attached to
}

// IsEmpty reports whether c is "empty", meaning it has no non-empty comment
// text for its associated value.
func (c Comments) IsEmpty() bool {
	return len(c.Before) == 0 && c.Line == "" && len(c.End) == 0
}

// Clear discards the contents of c, leaving it empty.
func (c *Comments) Clear() { c.Before = nil; c.Line = ""; c.End = nil }

// ValueLocation reports the location of the specified value.
// Values constructed rather than parsed may have a zero location.
func ValueLocation(v Value) jtree.Location { return v.Comments().vloc }

// Parse parses and returns a single JWCC value from r.  If r contains data
// after the first value, apart from comments and whitespace, Parse returns the
// first value along with an ast.ErrExtraInput error.
func Parse(r io.Reader) (*Document, error) {
	st := jtree.NewStream(r)
	st.AllowComments(true)
	st.AllowTrailingCommas(true)

	h := &parseHandler{ic: make(jtree.Interner)}
	if err := st.ParseOne(h); err != nil {
		return nil, err
	} else if len(h.stk) != 1 {
		return nil, errors.New("incomplete value")
	}
	v := h.stk[0]
	d := &Document{Value: v}
	vc := v.Comments()
	d.com.vloc.Span.End = vc.vloc.Span.End
	d.com.vloc.Last = vc.vloc.Last
	h.stk[0] = d
	if !h.eof {
		err := st.ParseOne(h)
		_, loc, com := h.consumeComments()
		d.com.End = com
		d.com.vloc.Span.End = loc.Span.End
		d.com.vloc.Last = loc.Last
		if err != nil && !errors.Is(err, io.EOF) {
			return d, errors.Join(ast.ErrExtraInput, err)
		}
	}
	return d, nil
}

// parseHandler implements the jtree.Handler interface for JWCC values.
type parseHandler struct {
	stk []Value
	ic  jtree.Interner
	eof bool
}

func (h *parseHandler) BeginObject(loc jtree.Anchor) error {
	h.pushValue(loc, &objectStub{})
	return nil
}

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	_, _, com := h.consumeComments() // trailing comments at the end of the object
	for i := len(h.stk) - 1; i >= 0; i-- {
		if stub, ok := h.stk[i].(*objectStub); ok {
			oloc := loc.Location()
			sc := stub.Comments()
			sc.End = com
			sc.vloc.Span.End = oloc.Span.End
			sc.vloc.Last = oloc.Last

			ms := make([]*Member, 0, len(h.stk)-i-1)
			for _, m := range h.stk[i+1:] {
				ms = append(ms, m.(*Member))
			}
			h.stk = h.stk[:i+1]
			h.stk[i] = &Object{
				Members: ms,
				com:     *sc,
			}
			return nil
		}
	}
	panic("unbalanced EndObject")
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error {
	h.pushValue(loc, &arrayStub{})
	return nil
}

func (h *parseHandler) EndArray(loc jtree.Anchor) error {
	_, _, com := h.consumeComments()
	for i := len(h.stk) - 1; i >= 0; i-- {
		if stub, ok := h.stk[i].(*arrayStub); ok {
			aloc := loc.Location()
			sc := stub.Comments()
			sc.End = com
			sc.vloc.Span.End = aloc.Span.End
			sc.vloc.Last = aloc.Last

			vals := make([]Value, len(h.stk)-i-1)
			copy(vals, h.stk[i+1:])
			h.stk = h.stk[:i+1]

			h.stk[i] = &Array{
				Values: vals,
				com:    *sc,
			}
			return nil
		}
	}
	panic("unbalanced EndArray")
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	// Note: Because we have not shifted the key, the stack already has all the
	// comments that occurred in the input before the colon separator.
	// We move them all above the key when recording.

	h.pushValue(loc, &Member{Key: ast.Quoted(h.ic.Intern(loc.Text()))})
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error {
	// Stack: ... [incomplete-member] [value] [comment...]
	_, _, com := h.consumeComments()
	n := len(h.stk)
	m := h.stk[n-2].(*Member)
	mloc := loc.Location()
	mc := m.Comments()
	mc.End = com
	mc.vloc.Last = mloc.Last
	m.Value = h.stk[n-1]
	mc.vloc.Span.End = m.Value.Comments().vloc.Span.End
	h.stk = h.stk[:n-1]
	// Stack: ... [complete-member: value]
	return nil
}

func (h *parseHandler) Value(loc jtree.Anchor) error {
	v, err := ast.AnchorValue(loc)
	if err != nil {
		return err
	}
	h.pushValue(loc, &Datum{Value: v})
	return nil
}

func (h *parseHandler) EndOfInput(loc jtree.Anchor) { h.eof = true }

func (h *parseHandler) Comment(loc jtree.Anchor) { h.pushComment(loc) }

/*
  Attachment rules for comments:

  Comments are associated with each grammar phrase based on source location.

  The BEFORE comments of a phrase are all those ending before the start of its
  span and starting at or after the end of the previous token.

  The LINE comment of a phrase, if it exists, is the unique line-ending ("//")
  comment starting on the same line as the end of the phrase.

  The END comments of a phrase are comments that occur at the end of the phrase
  that were not claimed by any other substructure of the phrase. This applies
  to arrays, objects, object members, and documents.

  When the parser encounters a comment token:

  - If the top of the stack is a complete grammar phrase (not a comment, object
    stub, array stub, or incomplete object member) and the new token is a line
    comment on the same line a the end of that phrase, the new token is
    attached to the phrase as its line comment.

  - Otherwise, the comment is shifted.

  When a non-comment token is about to be shifted, the parser pops off all the
  comments atop the stack, joins them into a group, and records this group as
  the BEFORE comments of the token being shifted.

  When an complete array, object, or object member is about to be reduced, the
  parser pops of all the comments atop the stack, joins them into a group, and
  records this group as the END comments of the phrase being reduced.

  Any trailing unconsumed comments remaining in the input after parsing the
  value for a document are attached as its END comments.
*/

// consumeComments removes all comments from the top of the stack to form a
// group. It returns nil if no comments were found atop the stack, otherwise
// it returns the last line of the group.
func (h *parseHandler) consumeComments() (int, jtree.Location, []string) {
	var grp []string

	// As we scan comments, keep track of gaps between runs of comment lines and
	// inject blanks to preserve them.
	//
	// For example:
	//
	//    // one
	//    // two
	//
	//    // three
	//
	// returns ["// one", "// two", "", "// three"].
	var loc jtree.Location

	i, prev, last := len(h.stk)-1, -1, 0
	for i >= 0 {
		c, ok := h.stk[i].(commentStub)
		if !ok {
			break
		}
		if prev < 0 {
			loc = c.vloc
		} else {
			loc.Span.Pos = c.vloc.Span.Pos
			loc.First = c.vloc.First
		}

		// Record the last line of the topmost comment.
		if len(grp) == 0 {
			last = c.vloc.Last.Line
		}

		// If there is a gap, inject a blank..
		if c.vloc.Last.Line < prev {
			grp = append(grp, "")
		}
		prev = c.vloc.First.Line

		grp = append(grp, c.text)
		i--
	}
	if len(grp) == 0 {
		return 0, loc, nil // no comments found
	}
	if strings.HasSuffix(grp[len(grp)-1], "\n") {
		last-- // compensate for the newline at the end of a line comment
	}

	h.stk = h.stk[:i+1] // pop
	slices.Reverse(grp)
	return last, loc, grp
}

// pushComment handles a comment token by either adjoining it to the grammar
// phrase atop the stack as its line comment, or shifting it.
func (h *parseHandler) pushComment(loc jtree.Anchor) {
	if i := len(h.stk) - 1; i >= 0 && loc.Token() == jtree.LineComment {
		switch h.stk[i].(type) {
		case *arrayStub, *objectStub, commentStub:
			// skip; we don't attach line comments to internal stubs
		default:
			c := h.stk[i].Comments()
			if c.vloc.Last.Line == loc.Location().First.Line { // same line
				if m, ok := h.stk[i].(*Member); ok {
					if m.Value == nil {
						// The member is not complete, don't claim this comment.
					} else if mvc := m.Value.Comments(); mvc.Line != "" {
						// The member value already has a line comment; move that
						// comment to the member instead, and stack the new comment.
						c.Line = mvc.Line
						mvc.Line = ""
					} else {
						c.Line = string(loc.Text())
						return
					}
					// fall through and shift the comment
				} else {
					// Attach this as the line comment of the phrase.
					c.Line = string(loc.Text())
					return
				}
			}
		}
	}
	h.stk = append(h.stk, commentStub{
		text: string(loc.Text()),
		vloc: loc.Location(),
	})
}

// pushValue pushes v atop the stack after handling any pending comments.
func (h *parseHandler) pushValue(loc jtree.Anchor, v Value) {
	last, _, com := h.consumeComments() // do this first, it may update the stack
	c := v.Comments()
	vp := loc.Location()
	c.vloc = vp
	if len(com) != 0 && last+1 < vp.First.Line {
		com = append(com, "")
	}
	c.Before = com
	h.stk = append(h.stk, v)
}
