// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"fmt"
	"io"
	"strings"
)

// An Anchor represents a location in source text. The methods of an Anchor
// will report the location, token type, and contents of the anchor.
type Anchor interface {
	Location() Location // Returns the source location of the anchor
	Token() Token       // Returns the token type of the anchor
	Text() string       // Returns the unescaped text of the anchor
	Unescape() string   // Unescapes the text of the anchor
}

// A Handler handles events from parsing an input stream.  If a method reports
// an error, parsing stops and that error is returned to the caller.
// The parser ensures objects and arrays are correctly balanced.
//
// The Anchor argument to a Handler method is only valid for the duration of
// that method call. If the method needs to retain information about the
// location after it returns, it must copy the relevant data.
//
// The SyntaxError method is special: The parser calls it to report a syntax
// error, and terminates parsing regardless what it returns.
type Handler interface {
	// Begin a new object, whose open brace is at loc.
	BeginObject(loc Anchor) error

	// End the most-recently-opened object, whose close brace is at loc.
	EndObject(loc Anchor) error

	// Begin a new array, whose open bracket is at loc.
	BeginArray(loc Anchor) error

	// End the most-recently-opened array, whose close bracket is at loc.
	EndArray(loc Anchor) error

	// Begin a new object member, whose key is at loc.
	BeginMember(loc Anchor) error

	// End the current object member giving the location and type of the token
	// that terminated the member (either Comma or RBrace).
	EndMember(loc Anchor) error

	// Report a data value at the given location.
	Value(loc Anchor) error

	// Report a syntax error at the given location and err != nil.
	// If SyntaxError returns a non-nil error, that value replaces err and is
	// reported from the parser. Otherwise the parser exits reporting err.
	SyntaxError(loc Anchor, err error) error

	// EndOfInput reports the end of the input stream.
	EndOfInput(loc Anchor)
}

// Stream is a stream parser that consumes input and delivers events to a
// Handler corresponding with the structure of the input.
type Stream struct {
	s *Scanner
}

// NewStream constructs a new Stream that consumes input from r.
func NewStream(r io.Reader) *Stream { return &Stream{s: NewScanner(r)} }

func (s *Stream) recoverParseError(errp *error) {
	if serr := recover(); serr != nil {
		*errp = serr.(error)
	}
}

// Parse parses the input stream and delivers events to h until either an error
// occurs or the input is exhausted.
func (s *Stream) Parse(h Handler) (err error) {
	defer s.recoverParseError(&err)

	for {
		err := s.s.Next()
		if err == io.EOF {
			h.EndOfInput(s.s)
			return nil
		} else if err != nil {
			s.syntaxError(h, "invalid input: %w", err)
		}

		s.parseElement(h)
	}
}

// ParseOne parses a single value from the input stream and delivers events to
// h until the value is complete or an error occurs. If no further value is
// available from the input, ParseOne returns io.EOF.
func (s *Stream) ParseOne(h Handler) (err error) {
	defer s.recoverParseError(&err)

	if err := s.s.Next(); err == io.EOF {
		h.EndOfInput(s.s)
		return err
	} else if err != nil {
		s.syntaxError(h, "invalid input: %w", err)
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
		s.syntaxError(h, "unexpected %v", tok)
	default:
		s.syntaxError(h, "unknown token %v", tok)
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
		if tok == RBrace {
			s.checkError(h.EndMember(s.s))
			return // end of object
		}
		s.checkError(h.EndMember(s.s))
		s.advance(h, String) // advance to next key
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
		s.advance(h)
		s.parseElement(h)
	}
}

func (s *Stream) advance(h Handler, tokens ...Token) Token {
	if err := s.s.Next(); err != nil {
		s.syntaxError(h, "expected %v, got error: %w", tokLabel(tokens), err)
	}
	tok := s.s.Token()
	if len(tokens) != 0 && !tokOneOf(tok, tokens) {
		s.syntaxError(h, "expected %v, got %v", tokLabel(tokens), tok)
	}
	return tok
}

func (s *Stream) require(h Handler, token Token) {
	if tok := s.s.Token(); tok != token {
		s.syntaxError(h, "expected %v, got %v", token, tok)
	}
}

func (s *Stream) syntaxError(h Handler, msg string, args ...interface{}) {
	loc := s.s.Location().First
	err := fmt.Errorf("at %s: "+msg, append([]interface{}{loc}, args...)...)
	if herr := h.SyntaxError(s.s, err); herr != nil {
		panic(herr)
	}
	panic(err)
}

func (s *Stream) checkError(err error) {
	if err != nil {
		panic(err)
	}
}

// tokLabel makes a human-readable summary string for the given token types.
func tokLabel(tokens []Token) string {
	if len(tokens) == 0 {
		return "more input"
	} else if len(tokens) == 1 {
		return tokens[0].String()
	}
	last := len(tokens) - 1
	ss := make([]string, len(tokens)-1)
	for i, tok := range tokens[:last] {
		ss[i] = tok.String()
	}
	return strings.Join(ss, ", ") + " or " + tokens[last].String()
}

// tokOneOf reports whether cur is an element of tokens.
func tokOneOf(cur Token, tokens []Token) bool {
	for _, token := range tokens {
		if cur == token {
			return true
		}
	}
	return false
}
