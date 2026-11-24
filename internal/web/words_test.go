package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// oldWords are words no screen says: it is called pitboard, not 板 or
// ボード (クリップボード is another word).
var oldWords = []string{"板", "ボード"}

func oldWordIn(s string) string {
	s = strings.ReplaceAll(s, "クリップボード", "")
	for _, w := range oldWords {
		if strings.Contains(s, w) {
			return w
		}
	}
	return ""
}

// The page's own files (not the gallery's fixtures) and every string the
// Go code (cmd, internal: the page's texts, the server's, the terminal's)
// can show say pitboard, never 板 or ボード.
func TestNoOldWords(t *testing.T) {
	err := fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(p, "static/gallery/") || !(strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".html")) {
			return nil
		}
		b, err := staticFS.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if w := oldWordIn(line); w != "" {
				t.Errorf("%s:%d says %q: %s", p, i+1, w, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, root := range []string{"../../cmd", "../../internal"} {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) < 20 {
		t.Fatalf("only %d Go files found", len(files))
	}
	fset := token.NewFileSet()
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if l, ok := n.(*ast.BasicLit); ok && l.Kind == token.STRING {
				if s, err := strconv.Unquote(l.Value); err == nil {
					if w := oldWordIn(s); w != "" {
						t.Errorf("%s says %q: %s", fset.Position(l.Pos()), w, s)
					}
				}
			}
			return true
		})
	}
}
