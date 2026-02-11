// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
)

// An Anchor represents a location in source text. The methods of an Anchor
// will report the location, token type, and contents of the anchor.
type Anchor interface {
	Token() Token       // Returns the token type of the anchor
	Text() []byte       // Returns a view of the raw (undecoded) text of the anchor
	Copy() []byte       // Returns a copy of the raw text of the anchor
	Location() Location // Returns the full location of the anchor
}

// A Handler handles events from parsing an input stream.  If a method reports
// an error, parsing stops and that error is returned to the caller.
// The parser ensures objects and arrays are correctly balanced.
//
// The Anchor argument to a Handler method is only valid for the duration of
// that method call. If the method needs to retain information about the
// location after it returns, it must copy the relevant data.
type Handler interface {
	// Begin a new object, whose open brace is at loc.
	BeginObject(loc Anchor) error

	// End the most-recently-opened object, whose close brace is at loc.
	EndObject(loc Anchor) error

	// Begin a new array, whose open bracket is at loc.
	BeginArray(loc Anchor) error

	// End the most-recently-opened array, whose close bracket is at loc.
	EndArray(loc Anchor) error

	// Begin a new object member, whose key is at loc.  The text of the key is
	// still quoted; the handler is responsible for unescaping key values if the
	// plain string is required (see jtree.Unquote).
	BeginMember(loc Anchor) error

	// End the current object member giving the location and type of the token
	// that terminated the member (either Comma or RBrace).
	EndMember(loc Anchor) error

	// Report a data value at the given location. The type of the value can be
	// recovered from the token. String tokens are quoted.
	Value(loc Anchor) error

	// EndOfInput reports the end of the input stream.
	EndOfInput(loc Anchor)
}

// CommentHandler is an optional interface that a Handler may implement to
// handle comment tokens. If a handler implements this method and comments are
// enabled in the scanner, Comment will be called for each comment token that
// occurs in the input. If the handler does not provide this method, comments
// will be silently discarded.
type CommentHandler interface {
	// Process the line or block comment at the specified location.
	// Line comments include their leading "//" and trailing newline (if present).
	// Block comments include their leading "/*" and trailing "*/".
	Comment(loc Anchor)
}

// Stream is a stream parser that consumes input and delivers events to a
// Handler corresponding with the structure of the input.
type Stream struct {
	s      *Scanner
	tcomma bool // allow trailing commas in objects and arrays
}

// NewStream constructs a new Stream that consumes input from r.
func NewStream(r io.Reader) *Stream { return &Stream{s: NewScanner(r)} }

// NewStreamWithScanner constructs a new Stream that consumes input from s.
func NewStreamWithScanner(s *Scanner) *Stream { return &Stream{s: s} }

// AllowComments configures the scanner associated with s to report (true) or
// reject (false) comment tokens.
func (s *Stream) AllowComments(ok bool) { s.s.AllowComments(ok) }

// AllowTrailingCommas configures the parser to allow (true) or reject (false)
// trailing comments in objects and arrays.
func (s *Stream) AllowTrailingCommas(ok bool) { s.tcomma = ok }

func (s *Stream) recoverParseError(errp *error) {
	if serr := recover(); serr != nil {
		switch err := serr.(type) {
		case *SyntaxError:
			*errp = err
		case handlerError:
			*errp = err.error
		default:
			panic(serr)
		}
	}
}

// Parse parses the input stream and delivers events to h until either an error
// occurs or the input is exhausted. In case of a syntax error, the returned
// error has type [*SyntaxError].
func (s *Stream) Parse(h Handler) (err error) {
	defer s.recoverParseError(&err)

	for {
		err := s.nextToken(h)
		if err == io.EOF {
			h.EndOfInput(s.s)
			return nil
		} else if err != nil {
			s.syntaxError(err, "%v", err)
		}

		s.parseElement(h)
	}
}

// ParseOne parses a single value from the input stream and delivers events to
// h until the value is complete or an error occurs. If no further value is
// available from the input, ParseOne returns io.EOF. In case of a syntax
// error, the returned error has type [*SyntaxError].
func (s *Stream) ParseOne(h Handler) (err error) {
	defer s.recoverParseError(&err)

	if err := s.nextToken(h); err == io.EOF {
		h.EndOfInput(s.s)
		return err
	} else if err != nil {
		s.syntaxError(err, "%v", err)
	}
	s.parseElement(h)
	return nil
}

// parseElement consumes a single value of any type.
// Precondition: token != Invalid.
func (s *Stream) parseElement(h Handler) {
	switch tok := s.s.Token(); tok {
	case LBrace:
		s.checkError(h.BeginObject(s.s))
		s.parseMembers(h)
		s.require(h, RBrace)
		s.checkError(h.EndObject(s.s))
	case LSquare:
		s.checkError(h.BeginArray(s.s))
		s.parseElements(h)
		s.require(h, RSquare)
		s.checkError(h.EndArray(s.s))
	case Integer, Number, String, True, False, Null:
		s.checkError(h.Value(s.s))
	case RBrace, RSquare, Comma, Colon:
		s.syntaxError(nil, "unexpected %v", tok)
	default:
		s.syntaxError(nil, "unknown token %v", tok)
	}
}

// parseMembers consumes zero of more key:value object members.
// Precondition: token == LBrace.
// Postcondition: token == RBrace.
func (s *Stream) parseMembers(h Handler) {
	tok := s.advance(h, RBrace, String)
	if tok == RBrace {
		return // end of object
	}
	for {
		// Parse a single member: "key": value
		s.checkError(h.BeginMember(s.s))
		s.advance(h, Colon)
		s.advance(h)
		s.parseElement(h)

		// Check whether we have more members (",") or are done ("}").
		tok := s.advance(h, RBrace, Comma)
		s.checkError(h.EndMember(s.s))
		if tok == RBrace {
			return // end of object
		} else if s.tcomma {
			// If trailing commas are allowed and the next token is a close
			// bracket, consider this a valid end of the object. Otherwise, it
			// must be a key for a subsequent element.
			next := s.advance(h, String, RBrace)
			if next == RBrace {
				return // end of object with trailing comma
			}
		} else {
			s.advance(h, String) // advance to next key
		}
	}
}

// parseElements consumes zero or more comma-separated array values.
// Precondition: token == LSquare.
// Postcondition: token == RSquare.
func (s *Stream) parseElements(h Handler) {
	if tok := s.advance(h); tok == RSquare {
		return // end of array
	}
	s.parseElement(h)
	for {
		tok := s.advance(h, RSquare, Comma)
		if tok == RSquare {
			return // end of array
		}

		// If trailing commas are allowed and the next token is a close bracket,
		// consider this a valid end of the array; otherwise it will fail on the
		// next element
		if next := s.advance(h); s.tcomma && next == RSquare {
			return // end of array with trailing comma
		}
		s.parseElement(h)
	}
}

func (s *Stream) nextToken(h Handler) error {
	for s.s.Next() {
		// If we see a comment token, pass it to the handler if it implements
		// CommentHandler. Either way, discard the comment and fetch the next
		// available comment for the rest of the parser.
		if tok := s.s.Token(); tok == LineComment || tok == BlockComment {
			if ch, ok := h.(CommentHandler); ok {
				ch.Comment(s.s)
			}
			continue // skip to the next token for the parser
		}
		return nil
	}
	return cmp.Or(s.s.Err(), io.EOF)
}

func (s *Stream) advance(h Handler, tokens ...Token) Token {
	if err := s.nextToken(h); err != nil {
		s.syntaxError(err, "%v", tokLabel(tokens, err))
	}
	tok := s.s.Token()
	if len(tokens) != 0 && !tokOneOf(tok, tokens) {
		s.syntaxError(nil, "%v", tokLabel(tokens, tok))
	}
	return tok
}

func (s *Stream) require(h Handler, token Token) {
	if tok := s.s.Token(); tok != token {
		s.syntaxError(nil, "expected %v, got %v", token, tok)
	}
}

func (s *Stream) syntaxError(err error, msg string, args ...any) {
	panic(&SyntaxError{
		Location: s.s.Location().First,
		Message:  fmt.Sprintf(msg, args...),
		err:      err,
	})
}

func (s *Stream) checkError(err error) {
	if err != nil {
		panic(handlerError{err})
	}
}

type handlerError struct{ error }

func (h handlerError) Unwrap() error { return h.error }

// tokLabel makes a human-readable summary string for the given token types.
func tokLabel(tokens []Token, got any) string {
	if len(tokens) == 0 {
		return fmt.Sprint(got)
	}
	var exp string
	if len(tokens) == 1 {
		exp = tokens[0].String()
	} else {
		last := len(tokens) - 1
		ss := make([]string, len(tokens)-1)
		for i, tok := range tokens[:last] {
			ss[i] = tok.String()
		}
		exp = strings.Join(ss, ", ") + " or " + tokens[last].String()
	}
	return fmt.Sprintf("expected %s, got %v", exp, got)
}

// tokOneOf reports whether cur is an element of tokens.
func tokOneOf(cur Token, tokens []Token) bool {
	return slices.Contains(tokens, cur)
}

// SyntaxError is the concrete type of errors reported by the stream parser.
type SyntaxError struct {
	Location LineCol
	Message  string

	err error
}

// Error satisfies the error interface.
func (s *SyntaxError) Error() string {
	return fmt.Sprintf("at %s: %s", s.Location, s.Message)
}

// Unwrap supports error wrapping.
func (s *SyntaxError) Unwrap() error { return s.err }
