// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"
)

// UnescapeString decodes the JSON encoding of a string. This function is a
// shorthand for a type conversion and a call to UnescapeBytes.
func UnescapeString(src string) (string, error) {
	u, err := Unescape([]byte(src))
	if err != nil {
		return "", err
	}
	return string(u), nil
}

// Unescape decodes a byte slice containing the JSON encoding of a string. The
// input must have the enclosing double quotation marks already removed.
//
// Escape sequences are replaced with their unescaped equivalents. Invalid
// escapes are replaced by the Unicode replacement rune. DecodeString reports
// an error for an incomplete escape sequence.
func Unescape(src []byte) ([]byte, error) {
	if !bytes.ContainsRune(src, '\\') {
		return src, nil
	}

	dec := bytes.NewBuffer(make([]byte, 0, len(src)))
	for len(src) != 0 {
		i := bytes.IndexRune(src, '\\')
		if i < 0 {
			dec.Write(src)
			break
		}
		dec.Write(src[:i])

		// Decode the next rune after the escape to figure out what to
		// substitute. There should not be errors here, but if there are, insert
		// replacement runes (utf8.RuneError == '\ufffd').
		src = src[i+1:]
		if len(src) == 0 {
			return nil, errors.New("incomplete escape sequence")
		}
		r, n := utf8.DecodeRune(src)
		if n == 0 {
			n++
		}

		src = src[n:]
		switch r {
		case '"', '\\', '/':
			dec.WriteRune(r)
		case 'b':
			dec.WriteRune('\b')
		case 'f':
			dec.WriteRune('\f')
		case 'n':
			dec.WriteRune('\n')
		case 'r':
			dec.WriteRune('\r')
		case 't':
			dec.WriteRune('\t')
		case 'u':
			if len(src) < 4 {
				return nil, errors.New("incomplete Unicode escape")
			}
			v, err := parseHex(src[:4])
			if err != nil {
				dec.WriteRune(utf8.RuneError)
			} else {
				dec.WriteRune(rune(v))
			}
			src = src[4:]
		default:
			dec.WriteRune(utf8.RuneError)
		}
	}
	return dec.Bytes(), nil
}

func parseHex(data []byte) (int64, error) {
	var v int64
	for _, b := range data {
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
