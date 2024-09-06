// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

// Package escape handles quoting and unquoting of JSON strings.
package escape

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"go4.org/mem"
)

// Unquote decodes a byte slice containing the JSON encoding of a string. The
// input must have the enclosing double quotation marks already removed.
//
// Escape sequences are replaced with their unescaped equivalents. Invalid
// escapes are replaced by the Unicode replacement rune. Unquote reports an
// error for an incomplete escape sequence.
func Unquote(src mem.RO) ([]byte, error) {
	dec := make([]byte, 0, src.Len())
	i := mem.IndexByte(src, '\\')
	if i < 0 {
		dec = mem.Append(dec, src)
		return dec, nil
	}

	putByte := func(bs ...byte) { dec = append(dec, bs...) }
	putRune := func(r rune) {
		var buf [6]byte
		n := utf8.EncodeRune(buf[:], r)
		dec = append(dec, buf[:n]...)
	}
	for src.Len() != 0 {
		dec = mem.Append(dec, src.SliceTo(i))

		// Decode the next rune after the escape to figure out what to
		// substitute. There should not be errors here, but if there are, insert
		// replacement runes (utf8.RuneError == '\ufffd').
		src = src.SliceFrom(i + 1)
		if src.Len() == 0 {
			return nil, errors.New("incomplete escape sequence")
		}
		r, n := mem.DecodeRune(src)
		if n == 0 {
			n++
		}

		src = src.SliceFrom(n)
		switch r {
		case '"', '\\', '/':
			putByte(byte(r))
		case 'b':
			putByte('\b')
		case 'f':
			putByte('\f')
		case 'n':
			putByte('\n')
		case 'r':
			putByte('\r')
		case 't':
			putByte('\t')
		case 'u':
			if src.Len() < 4 {
				return nil, errors.New("incomplete Unicode escape")
			}
			v, err := parseHex(src.SliceTo(4))
			if err != nil {
				putRune(utf8.RuneError)
			} else {
				putRune(rune(v))
			}
			src = src.SliceFrom(4)
		default:
			putRune(utf8.RuneError)
		}

		// Look for the next escape sequence, and if one is not found we can blit
		// the rest of the input and go home.
		i = mem.IndexByte(src, '\\')
		if i < 0 {
			dec = mem.Append(dec, src)
			break
		}
	}
	return dec, nil
}

func parseHex(data mem.RO) (int64, error) {
	var v int64
	for i := 0; i < data.Len(); i++ {
		b := data.At(i)
		v <<= 4
		if '0' <= b && b <= '9' {
			v += int64(b - '0')
		} else if 'a' <= b && b <= 'f' {
			v += int64(b - 'a' + 10)
		} else if 'A' <= b && b <= 'F' {
			v += int64(b - 'A' + 10)
		} else {
			return 0, fmt.Errorf("invalid hex digit %q", b)
		}
	}
	return v, nil
}
