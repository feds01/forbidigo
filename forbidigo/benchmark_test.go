package forbidigo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// corpusEnv overrides the benchmark corpus with a directory of Go sources.
const corpusEnv = "FORBIDIGO_BENCH_DIR"

// parseCorpus parses every .go file below the given directories.
func parseCorpus(tb testing.TB, dirs ...string) (*token.FileSet, []*ast.File) {
	tb.Helper()

	fset := token.NewFileSet()
	var files []*ast.File
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && (entry.Name() == "testdata" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				// Some files in corpora deliberately don't parse.
				return nil
			}
			files = append(files, file)
			return nil
		})
		if err != nil {
			tb.Fatalf("could not walk %s: %v", dir, err)
		}
	}
	if len(files) == 0 {
		tb.Fatalf("no Go files found in %v", dirs)
	}
	return fset, files
}

func benchmarkCorpusDirs() []string {
	if dir := os.Getenv(corpusEnv); dir != "" {
		return []string{dir}
	}
	return []string{filepath.Join(runtime.GOROOT(), "src", "net")}
}

func BenchmarkRun(b *testing.B) {
	fset, files := parseCorpus(b, benchmarkCorpusDirs()...)
	nodes := make([]ast.Node, 0, len(files))
	for _, file := range files {
		nodes = append(nodes, file)
	}

	linter, err := NewLinter([]string{
		`^(fmt\.Print(|f|ln)|print|println)$`,
		`^httprate\.KeyBy(IP|RealIP)$# use something else`,
	})
	if err != nil {
		b.Fatal(err)
	}

	nopDebugLog := func(format string, args ...interface{}) {}

	for name, debugLog := range map[string]func(string, ...interface{}){
		"nil_debug_log": nil,
		"nop_debug_log": nopDebugLog,
	} {
		b.Run(name, func(b *testing.B) {
			runConfig := RunConfig{Fset: fset, DebugLog: debugLog}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := linter.RunWithConfig(runConfig, nodes...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
