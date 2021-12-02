// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// UnescapeString decodes the JSON encoding of a string. The input must have
// the enclosing double quotation marks already removed. Escape sequences are
// replaced with their unescaped equivalents. Invalid escapes are replaced by
// the Unicode replacement rune. DecodeString reports an error for an
// incomplete escape sequence.
func UnescapeString(src string) (string, error) {
	if !strings.ContainsRune(src, '\\') {
		return src, nil
	}

	dec := bytes.NewBuffer(make([]byte, 0, len(src)))
	for src != "" {
		i := strings.IndexRune(src, '\\')
		if i < 0 {
			dec.WriteString(src)
			break
		}
		dec.WriteString(src[:i])

		// Decode the next rune after the escape to figure out what to
		// substitute. There should not be errors here, but if there are, insert
		// replacement runes (utf8.RuneError == '\ufffd').
		src = src[i+1:]
		if src == "" {
			return "", errors.New("incomplete escape sequence")
		}
		r, n := utf8.DecodeRuneInString(src)
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
				return "", errors.New("incomplete Unicode escape")
			}
			v, err := strconv.ParseInt(src[:4], 16, 64)
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
	return dec.String(), nil
}
