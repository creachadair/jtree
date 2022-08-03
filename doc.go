// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package jtree implements a JSON scanner and parser.
//
// # Scanning
//
// The Scanner type implements a lexical scanner for JSON.  Construct a scanner
// from an io.Reader and call its Next method to iterate over the stream. Next
// advances to the next input token and returns nil, or reports an error:
//
//	s := jtree.NewScanner(input)
//	for s.Next() == nil {
//	   log.Printf("Next token: %v", s.Token())
//	}
//
// Next returns io.EOF when the input has been fully consumed. Any other error
// indicates an I/O or lexical error in the input.
//
//	if s.Err() != io.EOF {
//	   log.Fatalf("Scanning failed: %v", err)
//	}
//
// # Streaming
//
// The Stream type implements an event-driven stream parser for JSON.  The
// parser works by calling methods on a Handler value to report the structure
// of the input. In case of error, parsing is terminated and an error of
// concrete type *jtree.SyntaxError is returned.
//
// Construct a Stream from an io.Reader, and call its Parse method. Parse
// returns nil if the input was fully processed without error. If a Handler
// method reports an error, parsing stops and that error is returned.
//
//	s := jtree.NewStream(input)
//	if err := s.Parse(handler); err != nil {
//	   log.Fatalf("Parse failed: %v", err)
//	}
//
// To parse a single value from the front of the input, call ParseOne. This
// method returns io.EOF if no further values are available:
//
//	if err := s.ParseOne(handle); err == io.EOF {
//	   log.Print("No more input")
//	} else if err != nil {
//	   log.Printf("ParseOne failed: %v", err)
//	}
//
// # Handlers
//
// The Handler interface accepts parser events from a Stream. The methods of
// a handler correspond to the syntax of JSON values:
//
//	JSON type  | Methods                   | Description
//	---------- | ------------------------- | ---------------------------------
//	object     | BeginObject, EndObject    | { ... }
//	array      | BeginArray, EndArray      | [ ... ]
//	member     | BeginMember, EndMember    | "key": value
//	value      | Value                     | true, false, null, number, string
//	--         | EndOfInput                | end of input
//
// Each method is passed an Anchor value that can be used to retrieve location
// and type information. See the comments on the Handler type for the meaning
// of each method's anchor value. The Anchor passed to a handler method is only
// valid for the duration of that method call; the handler must copy any data
// it needs to retain beyond the lifetime of the call.
//
// The parser ensures that corresponding Begin and End methods are correctly
// paired, or that a SyntaxError is reported.
package jtree
