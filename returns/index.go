package returns

import "go/ast"

// funcHasSingleReturnVal returns true if func called by e has a
// single return value (and false if it has multiple return values).
func funcHasSingleReturnVal(e *ast.CallExpr) bool {
	if e, ok := e.Fun.(*ast.SelectorExpr); ok {
		if x, ok := e.X.(*ast.Ident); ok {
			// exempt some functions that are known to only return one value
			return (x.Name == "errors" && e.Sel.Name == "New") || (x.Name == "fmt" && e.Sel.Name == "Errorf")
		}
	}
	if id, ok := e.Fun.(*ast.Ident); ok && id.Obj != nil {
		if fn, ok := id.Obj.Decl.(*ast.FuncDecl); ok {
			return len(fn.Type.Results.List) == 1
		}
	}
	return false
}
