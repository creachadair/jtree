// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
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
	r   *bufio.Reader
	buf bytes.Buffer
	tok Token
	err error

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

		// Handle constants: true, false, null
		var want string
		switch ch {
		case 't':
			s.tok = True
			want = "true"
			err = s.scanName(ch)
		case 'f':
			s.tok = False
			want = "false"
			err = s.scanName(ch)
		case 'n':
			s.tok = Null
			want = "null"
			err = s.scanName(ch)
		default:
			return s.failf("unexpected %q", ch)
		}
		if err != nil {
			return err
		} else if got := s.buf.String(); got != want {
			return s.failf("unknown constant %q", got)
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

// Int64 returns the text of the current token as an int64, or 0 if the text
// cannot be parsed as an integer. If the current token is Integer this will
// always succeed.
func (s *Scanner) Int64() int64 {
	v, err := strconv.ParseInt(s.buf.String(), 10, 64)
	if err == nil {
		return v
	}
	return 0
}

// Float64 returns the text of the current token as a float64, or math.NaN if
// the text cannot be parsed as a float. If the current token is Integer or
// Number this will always succeed.
func (s *Scanner) Float64() float64 {
	v, err := strconv.ParseFloat(s.buf.String(), 64)
	if err == nil {
		return v
	}
	return math.NaN()
}

func (s *Scanner) scanString(open rune) error {
	// omit the quotes from the stored text
	var esc bool
	for {
		ch, err := s.rune()
		if err != nil {
			return s.fail(err)
		} else if ch == open && !esc {
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

	// N.B. This implementation deviates slightly from the JSON spec, which
	// disallows multi-digit integers with a leading zero (e.g., "013").  This
	// implementation accepts and ignores extra leading zeroes.

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
	ch, err := s.readWhile(isDigit)
	if err != nil {
		if err == io.EOF {
			s.tok = Integer
			return nil
		}
		return err
	}

	// If a decimal point follows, consume a fractional part.
	var isFloat bool
	if ch == '.' {
		s.buf.WriteRune(ch)
		ch, err = s.readWhile(isDigit)
		if err == io.EOF {
			s.tok = Number
			return nil
		} else if err != nil {
			return s.fail(err)
		}
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
	_, err = s.readWhile(isDigit)
	if err == io.EOF {
		s.tok = Number
		return nil
	} else if err != nil {
		return s.fail(err)
	}
	s.unrune()
	s.tok = Number
	return nil
}

func (s *Scanner) scanName(first rune) error {
	s.buf.WriteRune(first)
	_, err := s.readWhile(isNameRune)
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
func (s *Scanner) readWhile(f func(rune) bool) (rune, error) {
	for {
		ch, err := s.rune()
		if err != nil {
			return 0, err
		} else if !f(ch) {
			return ch, nil
		}
		s.buf.WriteRune(ch)
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

func (s *Scanner) setErr(err error) error {
	s.err = err
	return err
}

func (s *Scanner) fail(err error) error {
	return s.setErr(fmt.Errorf("offset %d: unexpected error: %w", s.end, err))
}

func (s *Scanner) failf(msg string, args ...interface{}) error {
	return s.setErr(fmt.Errorf("offset %d: "+msg, append([]interface{}{s.end}, args...)...))
}

func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\r' || ch == '\n' || ch == '\t'
}

func isNumStart(ch rune) bool { return ch == '-' || isDigit(ch) }
func isExpStart(ch rune) bool { return ch == '-' || ch == '+' || isDigit(ch) }
func isDigit(ch rune) bool    { return '0' <= ch && ch <= '9' }
func isNameRune(ch rune) bool { return ch >= 'a' && ch <= 'z' }

func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

var self = [...]Token{LBrace, RBrace, LSquare, RSquare, Comma, Colon}

func selfDelim(ch rune) (Token, bool) {
	i := strings.IndexRune("{}[],:", ch)
	if i >= 0 {
		return self[i], true
	}
	return Invalid, false
}
