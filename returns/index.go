package returns

import (
	"go/ast"
	"go/types"
)

// funcHasSingleReturnVal returns true if func called by e has a
// single return value (and false if it has multiple return values).
func funcHasSingleReturnVal(typeInfo *types.Info, e *ast.CallExpr) bool {
	// quick local pass
	if id, ok := e.Fun.(*ast.Ident); ok && id.Obj != nil {
		if fn, ok := id.Obj.Decl.(*ast.FuncDecl); ok {
			return len(fn.Type.Results.List) == 1
		}
	}

	if typeInfo != nil {
		// look up in type info
		typ := typeInfo.TypeOf(e)
		if _, ok := typ.(*types.Tuple); ok {
			return false
		}
		return true
	}

	// conservatively return false if we don't have type info
	return false
}
