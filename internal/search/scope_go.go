package search

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type funcScope struct {
	start uint32
	end   uint32
	name  string
}

func enrichGoScopes(root string, results []Result) {
	if len(results) == 0 {
		return
	}
	cache := make(map[string][]funcScope)
	for i := range results {
		r := &results[i]
		if !strings.HasSuffix(r.Path, ".go") {
			continue
		}
		scopes, ok := cache[r.Path]
		if !ok {
			scopes = parseGoFuncScopes(root, r.Path)
			cache[r.Path] = scopes
		}
		if len(scopes) == 0 {
			continue
		}
		line := r.MatchLine
		if line == 0 {
			line = r.StartLine
		}
		if line == 0 {
			continue
		}
		var best funcScope
		found := false
		for _, scope := range scopes {
			if line < scope.start || line > scope.end {
				continue
			}
			if !found || (scope.end-scope.start) < (best.end-best.start) {
				best = scope
				found = true
			}
		}
		if !found {
			continue
		}
		r.ScopeStartLine = best.start
		r.ScopeEndLine = best.end
		r.ScopeKind = "func"
		r.ScopeName = best.name
	}
}

func parseGoFuncScopes(root string, relPath string) []funcScope {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, abs, src, 0)
	if err != nil {
		return nil
	}
	scopes := make([]funcScope, 0, len(file.Decls))
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if start <= 0 || end <= 0 || end < start {
			continue
		}
		scopes = append(scopes, funcScope{
			start: uint32(start),
			end:   uint32(end),
			name:  fn.Name.Name,
		})
	}
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].start != scopes[j].start {
			return scopes[i].start < scopes[j].start
		}
		return scopes[i].end < scopes[j].end
	})
	return scopes
}
