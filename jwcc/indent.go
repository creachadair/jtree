// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// A Formatter carries the settings for pretty-printing JWCC values.
// A zero value is ready for use with default settings.
type Formatter struct{}

func (f Formatter) indent() string { return "  " }

func (f Formatter) maxLineItems() int { return 3 }

// Format renders a pretty-printed representation of v to w with default
// settings.
func Format(w io.Writer, v Value) error {
	var f Formatter
	return f.Format(w, v)
}

// FormatToString formats v to a string with default settings.
// In case of error in formatting, it returns an empty string.
func FormatToString(v Value) string {
	var buf bytes.Buffer
	if Format(&buf, v) != nil {
		return ""
	}
	return buf.String()
}

// Format renders a pretty-printed representation of v to w using the settings
// from f.
func (f Formatter) Format(w io.Writer, v Value) error {
	tw := tabwriter.NewWriter(w, 4, 4, 1, ' ', 0)
	f.formatValue(tw, v, "", "", true)
	return tw.Flush()
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

// formatValue writes a representation of v to w indented by indent.
// If lineCom is true, it renders a line-ending comment for v, if present.
func (f Formatter) formatValue(w writeFlusher, v Value, init, indent string, lineCom bool) {
	com := v.Comments()
	f.indentComments(w, com.Before, indent, true)
	switch t := v.(type) {
	case *Array:
		f.formatArray(w, t, init, indent)
	case *Datum:
		fmt.Fprint(w, init, t.JSON())
	case *Document:
		f.formatValue(w, t.Value, init, indent, lineCom)
		if ec := t.Comments().End; len(ec) != 0 {
			io.WriteString(w, "\n")
			f.indentComments(w, ec, indent, false)
		}
	case *Object:
		f.formatObject(w, t, init, indent)
	default:
		panic(fmt.Sprintf("unknown value type %T", v))
	}
	if lineCom && com.Line != "" {
		fmt.Fprint(w, indentComment(com.Line, "\t"), "\n")
	}
}

func (f Formatter) formatArray(w writeFlusher, a *Array, init, indent string) bool {
	if f.isBoring(a) {
		fmt.Fprint(w, init, "[")
		for i, v := range a.Values {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			// We know there can be no line comment, since the array is boring.
			f.formatValue(w, v, "", "", false)
		}
		io.WriteString(w, "]")
		return true
	}

	// Before comments were already written.
	fmt.Fprint(w, init, "[\n")
	adent := indent + f.indent()
	for _, v := range a.Values {
		f.formatValue(w, v, adent, adent, false)

		// Render a line comment (if there is one) outside the comma.
		if ln := v.Comments().Line; ln != "" {
			fmt.Fprint(w, ",", indentComment(ln, "\t"), "\n")
		} else {
			fmt.Fprint(w, ",\n")
		}
	}

	// Insert trailer comments.
	if ec := a.Comments().End; len(ec) != 0 {
		if len(a.Values) != 0 {
			io.WriteString(w, "\n")
		}
		f.indentComments(w, ec, adent, false)
	}
	w.Flush()
	fmt.Fprint(w, indent, "]")
	return false
}

func (f Formatter) formatObject(w writeFlusher, o *Object, init, indent string) bool {
	if f.isBoring(o) {
		fmt.Fprint(w, init, "{")
		for i, m := range o.Members {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			fmt.Fprint(w, m.Key.JSON(), ": ")
			// We know there can be no line comment, since the object is boring.
			f.formatValue(w, m.Value, "", "", false)
		}
		io.WriteString(w, "}")
		return true
	}

	// Before comments were already written.
	fmt.Fprint(w, init, "{\n")
	mdent := indent + f.indent()
	prevBoring, curBoring := true, true
	for i, m := range o.Members {
		// Leave extra space before the next member if either it or its
		// predecessor was non-boring.
		prevBoring, curBoring = curBoring, f.isBoring(m)

		if i != 0 && !(prevBoring && curBoring) {
			io.WriteString(w, "\n")
		}

		f.indentComments(w, m.Comments().Before, mdent, false)
		fmt.Fprint(w, mdent, m.Key.JSON(), f.objSep(m.Value))

		if len(m.Value.Comments().Before) == 0 {
			f.formatValue(w, m.Value, "", mdent, false)
		} else {
			io.WriteString(w, "\n")
			f.formatValue(w, m.Value, mdent, mdent, false)
		}

		// Render a line comment (if there is one) outside the comma.
		if ln := m.Comments().Line; ln != "" {
			fmt.Fprint(w, ",", indentComment(ln, "\t"), "\n")
		} else {
			fmt.Fprint(w, ",\n")
		}
		if ec := m.Comments().End; len(ec) != 0 {
			f.indentComments(w, ec, mdent, false)
		}
	}

	// Insert trailer comments.
	if ec := o.Comments().End; len(ec) != 0 {
		if len(o.Members) != 0 {
			io.WriteString(w, "\n")
		}
		f.indentComments(w, ec, mdent, false)
	}
	w.Flush()
	fmt.Fprint(w, indent, "}")
	return false
}

// objSep returns a key-value separator for the given value.
// Boring values get indented so they line up in columns;
// non-boring values are stapled directly to the key.
func (f Formatter) objSep(v Value) string {
	if f.isBoring(v) {
		return ":\t"
	}
	return ": "
}

// canInlineComment reports whether comments ss have simple enough structure
// that they can be rendered inline.
func (Formatter) canInlineComment(ss []string) bool {
	if len(ss) == 1 {
		return strings.HasPrefix(ss[0], "/*") && strings.Count(ss[0], "\n") == 0
	}
	return false
}

// isBoring reports whether v has a simple enough structure that it can be
// rendered on one line.
func (f Formatter) isBoring(v Value) bool {
	com := v.Comments()
	switch t := v.(type) {
	case *Array:
		if len(com.End) != 0 {
			return false
		}
		for i, v := range t.Values {
			if !f.isBoring(v) || len(v.Comments().Before) != 0 || i >= f.maxLineItems() {
				return false
			}
		}
		return true
	case *Datum:
		return t.Comments().IsEmpty()
	case *Member:
		return len(com.Before) == 0 && len(com.End) == 0 && f.isBoring(t.Value)
	case *Object:
		if len(com.Before) != 0 || len(com.End) != 0 {
			return false
		}
		if len(t.Members) == 1 {
			return t.Members[0].Comments().IsEmpty() && f.isBoring(t.Members[0].Value)
		}
		return len(t.Members) == 0
	default:
		return false
	}
}

func (f Formatter) indentComments(w writeFlusher, ss []string, indent string, inlineOK bool) {
	if inlineOK && f.canInlineComment(ss) {
		fmt.Fprint(w, indentComment(ss[0], indent), " ")
		return
	}
	for _, s := range ss {
		if s == "" {
			io.WriteString(w, "\n")
			continue
		}
		fmt.Fprint(w, indentComment(s, indent), "\n")
	}
}

// indentComment realigns comment text from s and indents it by indent.
func indentComment(s, indent string) string {
	tag, text := classifyComment(s)
	var lines []string

	if strings.Count(text, "\n") == 0 {
		// The comment is just one line and is already trimmed.
		switch tag {
		case "/*":
			return indent + "/* " + text + " */"
		case "//":
			return indent + "//" + text
		default:
			return indent + "// " + text
		}
	}

	// The comment has multiple lines, and lines after the first are possibly
	// indented.
	lines = strings.Split(text, "\n")
	outdentCommentLines(lines)

	// Apply the indent and (if necessary) comment markers.
	all := make([]string, 0, len(lines)+2)
	if tag == "/*" {
		all = append(all, indent+"/*")
	}
	for _, line := range lines {
		switch tag {
		case "/*":
			all = append(all, indent+" "+line)
		case "//":
			all = append(all, indent+"//"+line)
		default:
			all = append(all, indent+"// "+line)
		}
	}
	if tag == "/*" {
		all = append(all, indent+"*/")
	}
	return strings.Join(all, "\n")
}

func classifyComment(s string) (tag, text string) {
	ns := strings.TrimSpace(s)

	if tail, ok := strings.CutPrefix(ns, "//"); ok {
		return "//", tail
	}
	if tail, ok := strings.CutPrefix(ns, "/*"); ok {
		base := strings.TrimSuffix(tail, "*/")
		return "/*", strings.TrimSpace(base)
	}
	return "??", ns
}

// trimSpaceSuffix removes whitespace from the suffix of s.
func trimSpaceSuffix(s string) string { return strings.TrimSpace("|" + s)[1:] }

// outdentCommentLines modifies lines to remove the shortest prefix of leading
// indentation that can be removed to leave the text flush left, and any
// trailing whitespace. It returns the count of indentation characters removed.
// The first line is assumed to be already cleaned of leading whitespace.
func outdentCommentLines(lines []string) int {
	// Find the shortest common prefix of the lines that can be removed to
	// leave the text flush left. Note the first line is already flush,
	// because we already trimmed it.
	pfx := -1
	for i, line := range lines[1:] {
		var ns int
		for _, c := range line {
			if c != ' ' && c != '\t' {
				break
			}
			ns++
		}
		if i == 0 || ns < pfx {
			pfx = ns
		}
	}

	// Remove the common prefix and trailing whitespace from each line.
	// Note the first line is already flush left, so skip it.
	lines[0] = trimSpaceSuffix(lines[0])
	for i, line := range lines[1:] {
		lines[i+1] = trimSpaceSuffix(line[pfx:])
	}
	return pfx
}
