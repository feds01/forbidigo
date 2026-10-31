package forbidigo

import (
	"go/ast"
	"go/token"
)

// plainText returns what go/printer prints for an identifier or a selector
// chain of identifiers (a.b.c), without running the printer. It reports false
// for any other node, and for chains the printer would break across lines or
// whose positions it cannot trust, so that the caller falls back to it.
func plainText(fset *token.FileSet, node ast.Node) (string, bool) {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name, true
	case *ast.SelectorExpr:
		var buf [64]byte
		text, _, ok := appendChain(fset, buf[:0], node, 0)
		return string(text), ok
	default:
		return "", false
	}
}

// appendChain appends the text of expr to text. All identifiers must have a
// valid position on the same line, which is passed in and returned as line
// (0 until the first identifier).
func appendChain(fset *token.FileSet, text []byte, expr ast.Expr, line int) ([]byte, int, bool) {
	switch expr := expr.(type) {
	case *ast.Ident:
		return appendIdent(fset, text, expr, line)
	case *ast.SelectorExpr:
		text, line, ok := appendChain(fset, text, expr.X, line)
		if !ok || expr.Sel == nil {
			return text, line, false
		}
		return appendIdent(fset, append(text, '.'), expr.Sel, line)
	default:
		return text, line, false
	}
}

func appendIdent(fset *token.FileSet, text []byte, ident *ast.Ident, line int) ([]byte, int, bool) {
	if ident == nil || !ident.Pos().IsValid() {
		return text, line, false
	}

	identLine := fset.PositionFor(ident.Pos(), false).Line
	if identLine == 0 || (line != 0 && identLine != line) {
		return text, line, false
	}

	return append(text, ident.Name...), identLine, true
}
