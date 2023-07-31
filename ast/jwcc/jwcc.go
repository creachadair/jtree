// Package jwcc implements a parser for JSON With Commas and Comments (JWCC) as
// defined by https://nigeltao.github.io/blog/2021/json-with-commas-comments.html
package jwcc

import (
	"errors"
	"io"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/ast"
	"golang.org/x/exp/slices"
)

// A Value is a JSON value optionally decorated with comments.
type Value interface {
	ast.Value

	// Comments returns the comments annotating this value.
	Comments() *Comments
}

// Comments records the comments associated with a value.
type Comments struct {
	Before []string
	Line   string
	End    []string

	eline int
}

// IsEmpty reports whether c is "empty", meaning it has no non-empty comment
// text for its associated value.
func (c Comments) IsEmpty() bool {
	return len(c.Before) == 0 && c.Line == "" && len(c.End) == 0
}

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
	h.stk[0] = d
	if !h.eof {
		err := st.ParseOne(h)
		d.Comments().End = h.consumeComments()
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
	com := h.consumeComments() // trailing comments at the end of the object
	for i := len(h.stk) - 1; i >= 0; i-- {
		if stub, ok := h.stk[i].(*objectStub); ok {
			stub.Comments().End = com

			ms := make([]*Member, 0, len(h.stk)-i-1)
			for j := i + 1; j < len(h.stk); j++ {
				ms = append(ms, h.stk[j].(*Member))
			}
			h.stk = h.stk[:i+1]
			h.stk[i] = &Object{
				Members: ms,
				com:     *stub.Comments(),
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
	com := h.consumeComments()
	for i := len(h.stk) - 1; i >= 0; i-- {
		if stub, ok := h.stk[i].(*arrayStub); ok {
			stub.Comments().End = com

			vals := make([]Value, len(h.stk)-i-1)
			copy(vals, h.stk[i+1:])
			h.stk = h.stk[:i+1]

			h.stk[i] = &Array{
				Values: vals,
				com:    *stub.Comments(),
			}
			return nil
		}
	}
	panic("unbalanced EndArray")
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	// Note: Comments between the key and the colon are offset to above the key.
	h.pushValue(loc, &Member{
		Key: ast.NewQuoted(h.ic.Intern(loc.Text())),
	})
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error {
	com := h.consumeComments()
	n := len(h.stk)
	m := h.stk[n-2].(*Member)
	m.Comments().End = com
	m.Comments().eline = loc.Location().Last.Line
	m.Value = h.stk[n-1]
	h.stk = h.stk[:n-1]
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
// group. It returns nil if no comments were found atop the stack.
func (h *parseHandler) consumeComments() []string {
	var grp []string

	i := len(h.stk) - 1
	for i >= 0 {
		c, ok := h.stk[i].(commentStub)
		if !ok {
			break
		}
		grp = append(grp, c.text)
		i--
	}
	if len(grp) == 0 {
		return nil // no comments found
	}

	h.stk = h.stk[:i+1] // pop
	slices.Reverse(grp)
	return grp
}

// pushComment handles a comment token by either adjoining it to the grammar
// phrase atop the stack as its line comment, or shifting it.
func (h *parseHandler) pushComment(loc jtree.Anchor) {
	if i := len(h.stk) - 1; i >= 0 && loc.Token() == jtree.LineComment {
		switch h.stk[i].(type) {
		case *arrayStub, *objectStub:
			// skip; we don't attach line comments to these
		case commentStub:
			// not a commentable value
		default:
			if t, ok := h.stk[i].(*Member); !ok || t.Value != nil {
				c := h.stk[i].Comments()
				if c.eline == loc.Location().First.Line { // same line
					// Attach this as the line comment of the phrase.
					c.Line = string(loc.Text())
					return
				}
			}
		}
	}
	h.stk = append(h.stk, commentStub{
		text: string(loc.Text()),
		span: loc.Location().Span,
	})
}

// pushValue pushes v atop the stack after handling any pending comments.
func (h *parseHandler) pushValue(loc jtree.Anchor, v Value) {
	com := h.consumeComments() // do this first, it may update the stack
	c := v.Comments()
	c.Before = com
	c.eline = loc.Location().Last.Line

	// Otherwise, accumulate the value normally.
	h.stk = append(h.stk, v)
}
