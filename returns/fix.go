// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package returns

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
)

func fixReturns(fset *token.FileSet, f *ast.File, typeInfo *types.Info) error {
	// map of potentially incomplete return statements (that might
	// need fixing) to the FuncType of the return's enclosing FuncDecl
	// or FuncLit
	incReturns := map[*ast.ReturnStmt]*ast.FuncType{}

	// collect incomplete returns
	ast.Walk(visitor{returns: incReturns}, f)

	//	printIncReturnsVerbose(fset, incReturns)

IncReturnsLoop:
	for ret, ftyp := range incReturns {
		if ftyp.Results == nil {
			continue
		}

		numRVs := len(ret.Results)
		if numRVs == len(ftyp.Results.List) {
			// correct return arity
			continue
		}

		if numRVs == 0 {
			// skip naked returns (could be named return values)
			continue
		}

		if numRVs > len(ftyp.Results.List) {
			// too many return values; preserve and ignore
			continue
		}

		// skip if return value is a func call (whose multiple returns
		// might be expanded)
		if e, ok := ret.Results[0].(*ast.CallExpr); ok {
			if !funcHasSingleReturnVal(typeInfo, e) {
				continue
			}
		}

		// left-fill zero values
		zvs := make([]ast.Expr, len(ftyp.Results.List)-numRVs)
		for i, rt := range ftyp.Results.List[:len(zvs)] {
			zv := newZeroValueNode(rt.Type)
			if zv == nil {
				// be conservative; if we can't determine the zero
				// value, don't fill in anything
				continue IncReturnsLoop
			}
			zvs[i] = zv
		}
		ret.Results = append(zvs, ret.Results...)
	}

	return nil
}

func removeBareReturns(fset *token.FileSet, f *ast.File, typeInfo *types.Info) error {
	// map of return statements to the FuncType of the return's enclosing
	// FuncDecl or FuncLit
	incReturns := map[*ast.ReturnStmt]*ast.FuncType{}

	// collect returns
	ast.Walk(visitor{returns: incReturns}, f)

	//	printIncReturnsVerbose(fset, incReturns)

IncReturnsLoop:
	for ret, ftyp := range incReturns {
		if ftyp.Results == nil {
			continue
		}

		numRVs := len(ret.Results)
		if numRVs == len(ftyp.Results.List) {
			// correct return arity
			continue
		}

		if numRVs == 0 && len(ftyp.Results.List) > 0 {
			zvs := make([]ast.Expr, len(ftyp.Results.List))
			for i, rt := range ftyp.Results.List {
				if len(rt.Names) == 0 {
					continue IncReturnsLoop
				}
				zv := &ast.Ident{Name: rt.Names[0].Name}
				zvs[i] = zv
			}
			ret.Results = append(zvs, ret.Results...)
		}
	}

	return nil
}

type visitor struct {
	enclosing *ast.FuncType                     // innermost enclosing func
	returns   map[*ast.ReturnStmt]*ast.FuncType // potentially incomplete returns
}

func (v visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return v
	}
	switch n := node.(type) {
	case *ast.FuncDecl:
		return visitor{enclosing: n.Type, returns: v.returns}
	case *ast.FuncLit:
		return visitor{enclosing: n.Type, returns: v.returns}
	case *ast.ReturnStmt:
		v.returns[n] = v.enclosing
	}
	return v
}

// newZeroValueNode returns an AST expr representing the zero value of
// typ. If determining the zero value requires additional information
// (e.g., type-checking output), it returns nil.
func newZeroValueNode(typ ast.Expr) ast.Expr {
	switch v := typ.(type) {
	case *ast.Ident:
		switch v.Name {
		case "uint8", "uint16", "uint32", "uint64", "int8", "int16", "int32", "int64", "byte", "rune", "uint", "int", "uintptr":
			return &ast.BasicLit{Kind: token.INT, Value: "0"}
		case "float32", "float64":
			return &ast.BasicLit{Kind: token.FLOAT, Value: "0"}
		case "complex64", "complex128":
			return &ast.BasicLit{Kind: token.IMAG, Value: "0"}
		case "bool":
			return &ast.Ident{Name: "false"}
		case "string":
			return &ast.BasicLit{Kind: token.STRING, Value: `""`}
		case "error":
			return &ast.Ident{Name: "nil"}
		}
	case *ast.ArrayType:
		if v.Len == nil {
			// slice
			return &ast.Ident{Name: "nil"}
		}
		return &ast.CompositeLit{Type: v}
	case *ast.StarExpr:
		return &ast.Ident{Name: "nil"}
	}
	return nil
}

func printIncReturns(fset *token.FileSet, v map[*ast.ReturnStmt]*ast.FuncType) {
	for ret, ftyp := range v {
		fmt.Print("FUNC TYPE: ")
		printer.Fprint(os.Stdout, fset, ftyp)
		fmt.Print("   RETURN: ")
		printer.Fprint(os.Stdout, fset, ret)
		fmt.Println()
	}
}

func printIncReturnsVerbose(fset *token.FileSet, v map[*ast.ReturnStmt]*ast.FuncType) {
	for ret, ftyp := range v {
		fmt.Print("FUNC TYPE: ")
		ast.Print(fset, ftyp)
		fmt.Print("   RETURN: ")
		ast.Print(fset, ret)
		fmt.Println()
	}
}
