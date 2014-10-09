// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package returns implements a Go pretty-printer (like package "go/format")
// that also adds zero-value return values as necessary to incomplete return
// statements.
package returns

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// Options specifies options for processing files.
type Options struct {
	Fragment  bool // Accept fragment of a source file (no package statement)
	AllErrors bool // Report all errors (not just the first 10 on different lines)
}

// Process formats and adjusts returns for the provided file.
// If opt is nil the defaults are used.
func Process(filename string, src []byte, opt *Options) ([]byte, error) {
	if opt == nil {
		opt = &Options{}
	}

	fileSet := token.NewFileSet()
	file, adjust, err := parse(fileSet, filename, src, opt)
	if err != nil {
		return nil, err
	}

	if err := fixReturns(fileSet, file); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = printer.Fprint(&buf, fileSet, file)
	if err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if adjust != nil {
		out = adjust(src, out)
	}

	out, err = format.Source(out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parse parses src, which was read from filename,
// as a Go source file or statement list.
func parse(fset *token.FileSet, filename string, src []byte, opt *Options) (*ast.File, func(orig, src []byte) []byte, error) {
	parserMode := parser.ParseComments
	if opt.AllErrors {
		parserMode |= parser.AllErrors
	}

	// Try as whole source file.
	file, err := parser.ParseFile(fset, filename, src, parserMode)
	if err == nil {
		return file, nil, nil
	}
	// If the error is that the source file didn't begin with a
	// package line and we accept fragmented input, fall through to
	// try as a source fragment.  Stop and return on any other error.
	if !opt.Fragment || !strings.Contains(err.Error(), "expected 'package'") {
		return nil, nil, err
	}

	// If this is a declaration list, make it a source file
	// by inserting a package clause.
	// Insert using a ;, not a newline, so that the line numbers
	// in psrc match the ones in src.
	psrc := append([]byte("package main;"), src...)
	file, err = parser.ParseFile(fset, filename, psrc, parserMode)
	if err == nil {
		// If a main function exists, we will assume this is a main
		// package and leave the file.
		if containsMainFunc(file) {
			return file, nil, nil
		}

		adjust := func(orig, src []byte) []byte {
			// Remove the package clause.
			// Gofmt has turned the ; into a \n.
			src = src[len("package main\n"):]
			return matchSpace(orig, src)
		}
		return file, adjust, nil
	}
	// If the error is that the source file didn't begin with a
	// declaration, fall through to try as a statement list.
	// Stop and return on any other error.
	if !strings.Contains(err.Error(), "expected declaration") {
		return nil, nil, err
	}

	// If this is a statement list, make it a source file
	// by inserting a package clause and turning the list
	// into a function body.  This handles expressions too.
	// Insert using a ;, not a newline, so that the line numbers
	// in fsrc match the ones in src.
	fsrc := append(append([]byte("package p; func _() {"), src...), '}')
	file, err = parser.ParseFile(fset, filename, fsrc, parserMode)
	if err == nil {
		adjust := func(orig, src []byte) []byte {
			// Remove the wrapping.
			// Gofmt has turned the ; into a \n\n.
			src = src[len("package p\n\nfunc _() {"):]
			src = src[:len(src)-len("}\n")]
			// Gofmt has also indented the function body one level.
			// Remove that indent.
			src = bytes.Replace(src, []byte("\n\t"), []byte("\n"), -1)
			return matchSpace(orig, src)
		}
		return file, adjust, nil
	}

	// Failed, and out of options.
	return nil, nil, err
}

// containsMainFunc checks if a file contains a function declaration with the
// function signature 'func main()'
func containsMainFunc(file *ast.File) bool {
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			if f.Name.Name != "main" {
				continue
			}

			if len(f.Type.Params.List) != 0 {
				continue
			}

			if f.Type.Results != nil && len(f.Type.Results.List) != 0 {
				continue
			}

			return true
		}
	}

	return false
}

func cutSpace(b []byte) (before, middle, after []byte) {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n') {
		i++
	}
	j := len(b)
	for j > 0 && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\n') {
		j--
	}
	if i <= j {
		return b[:i], b[i:j], b[j:]
	}
	return nil, nil, b[j:]
}

// matchSpace reformats src to use the same space context as orig.
// 1) If orig begins with blank lines, matchSpace inserts them at the beginning of src.
// 2) matchSpace copies the indentation of the first non-blank line in orig
//    to every non-blank line in src.
// 3) matchSpace copies the trailing space from orig and uses it in place
//   of src's trailing space.
func matchSpace(orig []byte, src []byte) []byte {
	before, _, after := cutSpace(orig)
	i := bytes.LastIndex(before, []byte{'\n'})
	before, indent := before[:i+1], before[i+1:]

	_, src, _ = cutSpace(src)

	var b bytes.Buffer
	b.Write(before)
	for len(src) > 0 {
		line := src
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, src = line[:i+1], line[i+1:]
		} else {
			src = nil
		}
		if len(line) > 0 && line[0] != '\n' { // not blank
			b.Write(indent)
		}
		b.Write(line)
	}
	b.Write(after)
	return b.Bytes()
}
