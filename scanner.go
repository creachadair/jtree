// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"

	"go4.org/mem"
)

// Token is the type of a lexical token in the JSON grammar.
type Token byte

// Constants defining the valid Token values.
const (
	Invalid Token = iota // invalid token
	LBrace               // left brace "{"
	RBrace               // right brace "}"
	LSquare              // left square bracket "["
	RSquare              // right square bracket "]"
	Comma                // comma ","
	Colon                // colon ":"
	Integer              // number: integer with no fraction or exponent
	Number               // number with fraction and/or exponent
	String               // quoted string
	True                 // constant: true
	False                // constant: false
	Null                 // constant: null

	BlockComment // comment: /* ... */
	LineComment  // comment: // ... <LF>

	// Do not modify the order of these constants without updating the
	// self-delimiting token check below.
)

var tokenStr = [...]string{
	Invalid: "invalid token",
	LBrace:  `"{"`,
	RBrace:  `"}"`,
	LSquare: `"["`,
	RSquare: `"]"`,
	Comma:   `","`,
	Colon:   `":"`,
	Integer: "integer",
	Number:  "number",
	String:  "string",
	True:    "true",
	False:   "false",
	Null:    "null",

	BlockComment: "block commment",
	LineComment:  "line comment",
}

func (t Token) String() string {
	v := int(t)
	if v > len(tokenStr) {
		return tokenStr[Invalid]
	}
	return tokenStr[v]
}

// A Scanner reads lexical tokens from an input stream.  Each call to Next
// advances the scanner to the next token, or reports an error.
type Scanner struct {
	r        *bufio.Reader
	comments bool         // allow comments
	buf      bytes.Buffer // current token
	tbuf     [][]byte     // allocation pool
	tok      Token
	err      error

	pos, end int // start and end offsets of current token
	last     int // size in bytes of last-read input rune

	// Apparent line and column offsets (0-based)
	pline, pcol int
	eline, ecol int
}

// NewScanner constructs a new lexical scanner that consumes input from r.
func NewScanner(r io.Reader) *Scanner {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Scanner{r: br}
}

// AllowComments configures the scanner to report (true) or reject (false)
// comment tokens. Comments are a non-standard exension of the JSON spec.  If
// enabled, C++ style block comments (/* ... */) and line comments (// ...)
// are recognized and emitted as tokens.
func (s *Scanner) AllowComments(ok bool) { s.comments = ok }

// Next advances s to the next token of the input, or reports an error.
// At the end of the input, Next returns io.EOF.
func (s *Scanner) Next() error {
	s.buf.Reset()
	s.err = nil
	s.tok = Invalid
	s.pos, s.pline, s.pcol = s.end, s.eline, s.ecol

	for {
		ch, err := s.rune()
		if err == io.EOF {
			return s.setErr(err)
		} else if err != nil {
			return s.fail(err)
		}

		// Discard whitespace.
		if isSpace(ch) {
			s.pos, s.pline, s.pcol = s.end, s.eline, s.ecol
			if ch == '\n' {
				s.eline++
				s.ecol = 0
			}
			continue
		}

		// Handle punctuation.
		if t, ok := selfDelim(ch); ok {
			s.buf.WriteRune(ch)
			s.tok = t
			return nil
		}

		// Handle numbers.
		if isNumStart(ch) {
			return s.scanNumber(ch)
		}

		// Handle string values.
		if ch == '"' {
			return s.scanString(ch)
		}

		// Handle comments, if enabled.
		if ch == '/' && s.comments {
			return s.scanComment(ch)
		}

		// Handle constants: true, false, null
		var want mem.RO
		switch ch {
		case 't':
			s.tok = True
			want = mem.S("true")
			err = s.scanName(ch)
		case 'f':
			s.tok = False
			want = mem.S("false")
			err = s.scanName(ch)
		case 'n':
			s.tok = Null
			want = mem.S("null")
			err = s.scanName(ch)
		default:
			return s.failf("unexpected %q", ch)
		}
		if err != nil {
			return err
		} else if got := mem.B(s.buf.Bytes()); !got.Equal(want) {
			return s.failf("unknown constant %q", got.StringCopy())
		}
		return nil // OK, token is already set
	}
}

// Token returns the type of the current token.
func (s *Scanner) Token() Token { return s.tok }

// Err returns the last error reported by Next.
func (s *Scanner) Err() error { return s.err }

// Text returns the undecoded text of the current token.  The return value is
// only valid until the next call of Next. The caller must copy the contents of
// the returned slice if it is needed beyond that.
func (s *Scanner) Text() []byte { return s.buf.Bytes() }

// Copy returns a copy of the undecoded text of the current token.
func (s *Scanner) Copy() []byte { return s.copyOf(s.buf.Bytes()) }

// Span returns the location span of the current token.
func (s *Scanner) Span() Span { return Span{Pos: s.pos, End: s.end} }

// Location returns the complete location of the current token.
func (s *Scanner) Location() Location {
	return Location{
		Span:  s.Span(),
		First: LineCol{Line: s.pline + 1, Column: s.pcol},
		Last:  LineCol{Line: s.eline + 1, Column: s.ecol},
	}
}

func (s *Scanner) scanString(open rune) error {
	s.buf.WriteRune(open)
	var esc bool
	for {
		ch, err := s.rune()
		if err != nil {
			return s.fail(err)
		} else if ch == open && !esc {
			s.buf.WriteRune(ch)
			s.tok = String
			return nil
		}
		if esc {
			// We are awaiting the completion of a \-escape.
			switch ch {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				s.buf.WriteByte(byte(ch))
			case 'u':
				s.buf.WriteByte(byte(ch))
				if err := s.readHex4(); err != nil {
					return s.failf("invalid Unicode escape: %w", err)
				}
			default:
				return s.failf("invalid %q after escape", ch)
			}
			esc = false
		} else if ch < ' ' {
			return s.failf("unescaped control %q", ch)
		} else if ch > unicode.MaxRune {
			return s.failf("invalid Unicode rune %q", ch)
		} else {
			s.buf.WriteRune(ch)
			esc = ch == '\\'
		}
	}
}

func (s *Scanner) scanNumber(start rune) error {
	s.buf.WriteRune(start)

	if start == '-' {
		// If there is a leading sign, we need at least one digit.
		// Otherwise, we already have one in start.
		ch, err := s.require(isDigit, "digit")
		if err != nil {
			return err
		}
		s.buf.WriteRune(ch)
	}

	// Consume the remainder of an integer.
	_, ch, err := s.readWhile(isDigit)
	if err != nil {
		if err == io.EOF {
			s.tok = Integer
			return nil
		}
		return err
	}

	// Check for extra leading zeroes, which are disallowed by the JSON spec.
	// That is: 0.12 is OK, 01.2 is not.
	if hasExtraLeadingZeroes(s.buf.Bytes()) {
		return s.failf("extra leading zeroes")
	}

	// If a decimal point follows, consume a fractional part.
	var isFloat bool
	if ch == '.' {
		s.buf.WriteRune(ch)
		var nr int
		nr, ch, err = s.readWhile(isDigit)
		if err != nil && err != io.EOF {
			return s.fail(err)
		} else if nr == 0 {
			return s.failf("no digits after decimal point")
		}
		s.tok = Number
		isFloat = true
	}

	// If an exponent follows, consume it.
	if ch != 'E' && ch != 'e' {
		s.unrune()
		if isFloat {
			s.tok = Number
		} else {
			s.tok = Integer
		}
		return nil
	}

	s.buf.WriteRune(ch)
	ch, err = s.require(isExpStart, "sign or digit")
	if err != nil {
		return err
	}
	s.buf.WriteRune(ch)
	nr, _, err := s.readWhile(isDigit)
	if nr == 0 && (ch == '-' || ch == '+') {
		// It's OK to have no digits if the previous rune was not a sign,
		// otherwise we have to have at least one.
		return s.failf("missing exponent digits")
	} else if err == io.EOF {
		s.tok = Number
		return nil
	} else if err != nil {
		return s.fail(err)
	}
	s.unrune()
	s.tok = Number
	return nil
}

func (s *Scanner) scanComment(first rune) error {
	s.buf.WriteRune(first)
	ch, err := s.rune()
	if err != nil {
		return err
	}
	switch ch {
	case '/': // line comment to LF
		s.buf.WriteRune(ch)
		_, end, err := s.readWhile(isNotLF)
		if err == nil {
			s.buf.WriteRune(end)
		} else if err != io.EOF {
			return err
		}
		s.tok = LineComment
		return nil

	case '*': // block comment
		s.buf.WriteRune(ch)
		for {
			_, end, err := s.readWhile(isNotStar)
			if err != nil {
				return err
			}
			s.buf.WriteRune(end) // end == '*'

			// Check whether we have "*/", which would end the comment.
			next, err := s.rune()
			if err != nil {
				return err
			}
			s.buf.WriteRune(next)
			if next == '/' {
				s.tok = BlockComment
				return nil
			}

			// We saw "*" but not "/", so keep scanning for the end of the block.
		}

	default:
		s.unrune()
		return s.failf("invalid %q in comment", ch)
	}
}

func (s *Scanner) scanName(first rune) error {
	s.buf.WriteRune(first)
	_, _, err := s.readWhile(isNameRune)
	if err == io.EOF {
		return nil
	} else if err != nil {
		return s.fail(err)
	}
	s.unrune()
	return nil
}

func (s *Scanner) rune() (rune, error) {
	ch, nb, err := s.r.ReadRune()
	s.last = nb
	s.end += nb
	s.ecol += nb
	return ch, err
}

func (s *Scanner) unrune() {
	s.end -= s.last
	s.ecol -= s.last
	s.last = 0
	s.r.UnreadRune()
}

// require reads a single rune matching f from the input, or returns an error
// mentioning the desired label.
func (s *Scanner) require(f func(rune) bool, label string) (rune, error) {
	ch, err := s.rune()
	if err != nil {
		return 0, s.failf("want %s, got error: %w", label, err)
	} else if !f(ch) {
		s.unrune()
		return 0, s.failf("got %q, want %s", ch, label)
	}
	return ch, nil
}

// readWhile consumes runes matching f from the input until EOF or until a rune
// not matching f is found. The first non-matching rune (if any) is returned.
// It is the caller's responsibility to unread this rune, if desired.
// The int reports the number of runes consumed.
func (s *Scanner) readWhile(f func(rune) bool) (int, rune, error) {
	var nr int
	for {
		ch, err := s.rune()
		if err != nil {
			return nr, 0, err
		} else if !f(ch) {
			return nr, ch, nil
		}
		s.buf.WriteRune(ch)
		nr++
	}
}

// readHex4 reads exactly 4 hexadecimal digits from the input.
func (s *Scanner) readHex4() error {
	for i := 0; i < 4; i++ {
		ch, err := s.rune()
		if err != nil {
			return err
		} else if !isHexDigit(ch) {
			return fmt.Errorf("not a hex digit: %q", ch)
		}
		s.buf.WriteRune(ch)
	}
	return nil
}

type posError struct {
	pos int
	err error
}

func (p posError) Error() string {
	return fmt.Sprintf("%s (offset %d)", p.err.Error(), p.pos)
}

func (p posError) Unwrap() error { return p.err }

func (s *Scanner) setErr(err error) error {
	s.err = err
	return err
}

func (s *Scanner) fail(err error) error {
	return s.setErr(posError{s.end, err})
}

func (s *Scanner) failf(msg string, args ...any) error {
	return s.setErr(posError{s.end, fmt.Errorf(msg, args...)})
}

func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\r' || ch == '\n' || ch == '\t'
}

func isNotStar(ch rune) bool  { return ch != '*' }
func isNotLF(ch rune) bool    { return ch != '\n' }
func isNumStart(ch rune) bool { return ch == '-' || isDigit(ch) }
func isExpStart(ch rune) bool { return ch == '-' || ch == '+' || isDigit(ch) }
func isDigit(ch rune) bool    { return '0' <= ch && ch <= '9' }
func isNameRune(ch rune) bool { return ch >= 'a' && ch <= 'z' }

func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// hasExtraLeadingZeroes reports whether the representation of an integer in
// buf has redundant leading zeroes, disallowed by the spec.
//
// OK: 0, 0.1, -1.0, -0.1 are all OK.
// Bad: -01, 01.2, -01.0, 00.1.
func hasExtraLeadingZeroes(buf []byte) bool {
	if buf[0] == '-' {
		buf = buf[1:] // skip leading sign
	}
	if buf[0] == '0' {
		// A leading zero is OK if it's the only digit.
		return len(buf) > 1
	}
	return false
}

var self = [...]Token{LBrace, RBrace, LSquare, RSquare, Comma, Colon}

func selfDelim(ch rune) (Token, bool) {
	i := strings.IndexRune("{}[],:", ch)
	if i >= 0 {
		return self[i], true
	}
	return Invalid, false
}

func (s *Scanner) copyOf(text []byte) []byte {
	const minBlockSlop = 4
	const smallSizeFraction = 16
	const bufBlockBytes = 16384

	// For values bigger than smallSizeFraction of the block size, don't bother
	// batching, make an outright copy.
	if len(text) >= bufBlockBytes/smallSizeFraction {
		return append([]byte(nil), text...)
	}

	// Look for a block with space enough to hold a copy of text.
	i := 0
	for i < len(s.tbuf) {
		if n := len(s.tbuf[i]) + len(text); n < cap(s.tbuf[i]) {
			// There is room in this block.
			break
		} else if cap(s.tbuf[i])-len(text) < minBlockSlop {
			// There is no room in this block, but it is nearly-enough full.
			// Allocate a fresh block at this location and release the old one.
			// The old block will be retained until all its tokens are released.
			s.tbuf[i] = make([]byte, 0, bufBlockBytes)
			break
		}
		i++
	}
	if i == len(s.tbuf) {
		// No block had room; add a new empty one to the arena.
		s.tbuf = append(s.tbuf, make([]byte, 0, bufBlockBytes))
	}
	p := len(s.tbuf[i])
	s.tbuf[i] = append(s.tbuf[i], text...)
	return s.tbuf[i][p : p+len(text)]
}
