package forbidigo

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func printedText(t *testing.T, fset *token.FileSet, node ast.Node) string {
	t.Helper()

	buf := new(bytes.Buffer)
	require.NoError(t, printer.Fprint(buf, fset, node))
	return buf.String()
}

// expectPlainTextMatchesPrinter checks every identifier and selector in files.
// It returns how many nodes took the fast path and how many were checked.
func expectPlainTextMatchesPrinter(t *testing.T, fset *token.FileSet, files []*ast.File) (fast, total int) {
	t.Helper()

	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.Ident, *ast.SelectorExpr:
			default:
				return true
			}

			total++
			text, ok := plainText(fset, node)
			if !ok {
				return true
			}

			fast++
			if expected := printedText(t, fset, node); text != expected {
				t.Fatalf("%s: plainText = %q, printer = %q", fset.Position(node.Pos()), text, expected)
			}
			return true
		})
	}
	return fast, total
}

func TestPlainTextMatchesPrinter(t *testing.T) {
	t.Run("in the stdlib and forbidigo's own sources", func(t *testing.T) {
		goSrc := filepath.Join(runtime.GOROOT(), "src")
		fset, files := parseCorpus(t,
			filepath.Join(goSrc, "go"),
			filepath.Join(goSrc, "net"),
			filepath.Join(goSrc, "reflect"),
			filepath.Join(goSrc, "encoding"),
			filepath.Join(goSrc, "strings"),
			filepath.Join(goSrc, "fmt"),
			filepath.Join(goSrc, "os"),
			filepath.Join(goSrc, "slices"),
			filepath.Join(goSrc, "maps"),
			filepath.Join(goSrc, "iter"),
			filepath.Join(goSrc, "sync"),
			filepath.Join(goSrc, "text", "template"),
			filepath.Join("..", "pkg"),
			filepath.Join("..", "examples"),
		)

		fast, total := expectPlainTextMatchesPrinter(t, fset, files)
		t.Logf("fast path used for %d of %d nodes in %d files", fast, total, len(files))
		assert.True(t, fast*10 > total*9, "fast path should cover most nodes")
	})

	t.Run("in tricky expressions", func(t *testing.T) {
		const src = `package p

import "fmt"

func f[T any, U ~[]T](s *S[T], u U) {
	fmt.Println(a.b.c, (a).b, (*p).x, *a.b, &a.b, a.b[0].c, a.b().c)
	fmt.
		Println(a.
			b, a.b.
			c)
	g[int].h()
	var _ = pkg.Generic[int]
	var _ pkg.Type[a.B, *c.D]
	x := struct{ a.b }{}
	_ = a /* comment */ . /* comment */ b
	_ = a. // comment
		b
	_ = ünï.çödé
	_ = _._
	_ = a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.aa.bb.cc.dd.ee.ff.gg.hh.ii
	_ = a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.
		aa.bb.cc.dd.ee.ff.gg.hh.ii
}
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "tricky.go", src, parser.ParseComments)
		require.NoError(t, err)

		fast, total := expectPlainTextMatchesPrinter(t, fset, []*ast.File{file})
		assert.True(t, fast > 0)
		assert.True(t, fast < total, "some nodes should fall back to the printer")
	})

	t.Run("falls back for chains the printer breaks across lines", func(t *testing.T) {
		const src = `package p

var _ = a.
	b.
	c
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "multiline.go", src, 0)
		require.NoError(t, err)

		selector := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]
		_, ok := plainText(fset, selector)
		assert.False(t, ok)
		assert.Equal(t, "a.\n\tb.\n\tc", printedText(t, fset, selector))
	})

	t.Run("falls back for nodes without positions", func(t *testing.T) {
		fset := token.NewFileSet()
		selector := &ast.SelectorExpr{X: ast.NewIdent("a"), Sel: ast.NewIdent("b")}

		_, ok := plainText(fset, selector)
		assert.False(t, ok)
		assert.Equal(t, "a.b", printedText(t, fset, selector))
	})

	t.Run("falls back for other nodes", func(t *testing.T) {
		fset := token.NewFileSet()
		call := &ast.CallExpr{Fun: ast.NewIdent("f")}

		_, ok := plainText(fset, call)
		assert.False(t, ok)
	})
}
