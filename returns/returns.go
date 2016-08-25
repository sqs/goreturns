// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package returns implements a Go pretty-printer (like package "go/format")
// that also adds zero-value return values as necessary to incomplete return
// statements.
package returns

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// Options specifies options for processing files.
type Options struct {
	Fragment bool // Accept fragment of a source file (no package statement)

	PrintErrors bool // Print non-fatal typechecking errors to stderr (interferes with some tools that use gofmt/goimports and expect them to only print code or diffs to stdout + stderr)

	AllErrors bool // Report all errors (not just the first 10 on different lines)

	RemoveBareReturns bool // Remove bare returns
}

// Process formats and adjusts returns for the provided file in a
// package in pkgDir. If pkgDir is empty, the file is treated as a
// standalone fragment (opt.Fragment should be true). If opt is nil
// the defaults are used.
func Process(pkgDir, filename string, src []byte, opt *Options) ([]byte, error) {
	if opt == nil {
		opt = &Options{}
	}

	fileSet := token.NewFileSet()
	file, adjust, typeInfo, err := parseAndCheck(fileSet, pkgDir, filename, src, opt)
	if err != nil {
		return nil, err
	}

	if err := fixReturns(fileSet, file, typeInfo); err != nil {
		return nil, err
	}

	if opt.RemoveBareReturns {
		if err := removeBareReturns(fileSet, file, typeInfo); err != nil {
			return nil, err
		}
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

func parseAndCheck(fset *token.FileSet, pkgDir, filename string, src []byte, opt *Options) (*ast.File, func(orig, src []byte) []byte, *types.Info, error) {
	var pkgFiles []*ast.File // all package files

	// Parse the named file using `parse`, which handles fragments and reads from the src byte array.
	file, adjust, err := parse(fset, filename, src, opt)
	if err != nil {
		return nil, nil, nil, err
	}
	pkgFiles = append(pkgFiles, file)

	var importPath string
	if pkgDir != "" {
		// Parse other package files by reading from the filesystem.
		dir := filepath.Dir(filename)
		buildPkg, err := build.ImportDir(dir, 0)
		if err != nil {
			// TODO(sqs): support parser-only mode (that doesn't require
			// files passed to goreturns to be part of a valid package)
			return nil, nil, nil, err
		}
		importPath = buildPkg.ImportPath
		for _, files := range [...][]string{buildPkg.GoFiles, buildPkg.CgoFiles} {
			for _, file := range files {
				if file == filepath.Base(filename) {
					// already parsed this file above
					continue
				}
				f, err := parser.ParseFile(fset, filepath.Join(dir, file), nil, 0)
				if err != nil {
					fmt.Fprintf(os.Stderr, "could not parse %q: %v\n", file, err)
					continue
				}
				pkgFiles = append(pkgFiles, f)
			}
		}
	}

	var nerrs int
	cfg := types.Config{
		Error: func(err error) {
			if opt.PrintErrors && (opt.AllErrors || nerrs == 0) {
				fmt.Fprintln(os.Stderr, err)
			}
			nerrs++
		},
	}

	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Uses:  map[*ast.Ident]types.Object{},
		Defs:  map[*ast.Ident]types.Object{},
	}
	if _, err := cfg.Check(importPath, fset, pkgFiles, info); err != nil {
		if terr, ok := err.(types.Error); ok && strings.HasPrefix(terr.Msg, "wrong number of return values") {
			// ignore "wrong number of return values" errors
		} else {
			if opt.PrintErrors {
				fmt.Fprintf(os.Stderr, "%s: typechecking failed (continuing without type info)\n", filename)
			}
			// proceed but without type info
			return file, adjust, nil, nil
		}
	}

	return file, adjust, info, nil
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
