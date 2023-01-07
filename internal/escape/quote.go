// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package escape

import (
	"unicode/utf8"

	"go4.org/mem"
)

var controlEsc = [...]byte{
	'\b': 'b',
	'\f': 'f',
	'\n': 'n',
	'\r': 'r',
	'\t': 't',
	' ':  ' ', // sentinel
}

var hexDigit = []byte("0123456789abcdef")

// Quote encodes a string to escape characters for inclusion in a JSON string.
func Quote(src mem.RO) []byte {
	buf := make([]byte, 0, src.Len())
	putByte := func(bs ...byte) { buf = append(buf, bs...) }

	i := 0
	for i < src.Len() {
		r, n := mem.DecodeRune(src)
		if r < utf8.RuneSelf {
			if r < ' ' {
				if b := controlEsc[r]; b != 0 {
					putByte('\\', b)
				} else {
					putByte('\\', 'u', '0', '0', hexDigit[int(r>>4)], hexDigit[int(r&15)])
				}
			} else if r == '\\' || r == '"' {
				putByte('\\', byte(r))
			} else {
				putByte(byte(r))
			}
			src = src.SliceFrom(n)
			continue
		}

		switch r {
		case '\ufffd': // replacement rune
			buf = append(buf, `\ufffd`...)
		case '\u2028': // line separator
			buf = append(buf, `\u2028`...)
		case '\u2029': // paragraph separator
			buf = append(buf, `\u2029`...)
		default:
			var rbuf [6]byte
			n := utf8.EncodeRune(rbuf[:], r)
			buf = append(buf, rbuf[:n]...)
		}

		src = src.SliceFrom(n)
	}
	return buf
}
