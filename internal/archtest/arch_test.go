package archtest

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const module = "anvil"

type pkg struct {
	ImportPath string
	Dir        string
	Name       string
	GoFiles    []string
	Imports    []string
}

// listPackages returns every non-test package in the module, for the host OS and for the
// other supported OS, so imports in _windows.go and _unix.go files are both checked.
func listPackages(t *testing.T) []pkg {
	t.Helper()
	seen := map[string]*pkg{}
	var order []string
	for _, goos := range []string{"linux", "windows"} {
		cmd := exec.Command("go", "list", "-json", "./...")
		cmd.Dir = filepath.Join("..", "..")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list (GOOS=%s): %v", goos, err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for dec.More() {
			var p pkg
			if err := dec.Decode(&p); err != nil {
				t.Fatal(err)
			}
			if prev, ok := seen[p.ImportPath]; ok {
				prev.Imports = append(prev.Imports, p.Imports...)
				prev.GoFiles = append(prev.GoFiles, p.GoFiles...)
				continue
			}
			seen[p.ImportPath] = &p
			order = append(order, p.ImportPath)
		}
	}
	out := make([]pkg, 0, len(order))
	for _, ip := range order {
		p := seen[ip]
		p.GoFiles = unique(p.GoFiles)
		p.Imports = unique(p.Imports)
		out = append(out, *p)
	}
	return out
}

func unique(s []string) []string {
	m := map[string]bool{}
	var out []string
	for _, v := range s {
		if !m[v] {
			m[v] = true
			out = append(out, v)
		}
	}
	return out
}

// layers maps package path prefixes (relative to the module) to their layer. Higher numbers
// may import lower ones, never the reverse (docs/reference/14 §2). The longest prefix wins.
var layers = map[string]int{
	"internal/errs": 0, "internal/version": 0, "internal/config": 0, "internal/logging": 0,
	"internal/fsutil": 0, "internal/service": 0,

	"internal/store": 1, "internal/git": 1, "internal/blob": 1, "internal/events": 1,
	"internal/queue": 1, "internal/ci/model": 1, "internal/ci/parser": 1, "internal/ci/expr": 1,
	"internal/ci/logs": 1,

	"internal/auth": 2, "internal/users": 2, "internal/repos": 2, "internal/issues": 2,
	"internal/pulls": 2, "internal/ci/trigger": 2, "internal/ci/scheduler": 2,
	"internal/artifacts": 2, "internal/releases": 2, "internal/packages": 2,
	"internal/deployments": 2, "internal/secrets": 2, "internal/explorer": 2,

	"internal/httpserver": 3, "internal/api": 3, "internal/git/smarthttp": 3,
	"internal/git/hooks": 3, "internal/ci/runnerapi": 3, "web": 3,

	"cmd": 4, "tools": 4,
}

// serviceLayer is the layer whose packages must not import net/http.
const serviceLayer = 2

// httpAllowedInServices lists service packages that legitimately serve HTTP themselves.
var httpAllowedInServices = map[string]bool{"internal/explorer": true}

var execAllowed = []string{
	"internal/git/gitcmd", "internal/service", "internal/runner/procgroup",
	"internal/runner/shell", "internal/runner/checkout", "tools/",
}

var runnerMayImport = []string{"internal/runner", "internal/ci/model", "internal/ci/expr", "internal/fsutil", "internal/version", "internal/errs"}

func rel(importPath string) (string, bool) {
	if importPath == module {
		return "", true
	}
	return strings.CutPrefix(importPath, module+"/")
}

func layerOf(p string) (int, bool) {
	best, layer := -1, 0
	for prefix, l := range layers {
		if (p == prefix || strings.HasPrefix(p, prefix+"/")) && len(prefix) > best {
			best, layer = len(prefix), l
		}
	}
	return layer, best >= 0
}

func hasPrefixAny(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		if p == pre || strings.HasPrefix(p, strings.TrimSuffix(pre, "/")+"/") {
			return true
		}
	}
	return false
}

func TestDependencyRules(t *testing.T) {
	for _, p := range listPackages(t) {
		me, _ := rel(p.ImportPath)
		if me == "internal/archtest" {
			continue
		}
		myLayer, known := layerOf(me)
		if !known && !strings.HasPrefix(me, "internal/runner") {
			t.Errorf("%s has no layer: add it to archtest.layers and docs/reference/14 §2", me)
		}
		for _, imp := range p.Imports {
			switch {
			case imp == "database/sql" && me != "internal/store":
				t.Errorf("%s imports database/sql; only internal/store may", me)
			case imp == "os/exec" && !hasPrefixAny(me, execAllowed):
				t.Errorf("%s imports os/exec; only %v may", me, execAllowed)
			case imp == "net/http" && known && myLayer == serviceLayer && !httpAllowedInServices[me]:
				t.Errorf("service package %s imports net/http", me)
			}
			dep, internal := rel(imp)
			if !internal {
				continue
			}
			if strings.HasPrefix(me, "internal/runner") {
				if !hasPrefixAny(dep, runnerMayImport) {
					t.Errorf("runner package %s imports server package %s", me, dep)
				}
				continue
			}
			if depLayer, ok := layerOf(dep); ok && known && depLayer > myLayer {
				t.Errorf("%s (layer %d) imports higher-layer %s (layer %d)", me, myLayer, dep, depLayer)
			}
		}
	}
}

// TC-NFR-09: every package has doc.go with a package comment; every exported identifier
// has a doc comment.
func TestDocumentation(t *testing.T) {
	for _, p := range listPackages(t) {
		me, _ := rel(p.ImportPath)
		fset := token.NewFileSet()
		docGo, err := parser.ParseFile(fset, filepath.Join(p.Dir, "doc.go"), nil, parser.ParseComments|parser.PackageClauseOnly)
		if p.Name != "main" && (err != nil || docGo.Doc == nil) {
			t.Errorf("%s: missing doc.go with a package comment", me)
		}
		for _, f := range p.GoFiles {
			file, err := parser.ParseFile(fset, filepath.Join(p.Dir, f), nil, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			for _, miss := range undocumented(file) {
				t.Errorf("%s/%s: exported %s has no doc comment", me, f, miss)
			}
		}
	}
}

// undocumented returns the exported top-level identifiers in f without a doc comment.
// Methods count only when their receiver type is exported. A documented group
// (const ( … ) with a comment above) documents all of its members.
func undocumented(f *ast.File) []string {
	var out []string
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() || d.Doc != nil {
				continue
			}
			if d.Recv != nil {
				if !receiverExported(d.Recv) || isInterfaceMethodName(d.Name.Name) {
					continue
				}
			}
			out = append(out, d.Name.Name)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() && d.Doc == nil && s.Doc == nil {
						out = append(out, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.IsExported() && d.Doc == nil && s.Doc == nil && s.Comment == nil {
							out = append(out, n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

func receiverExported(recv *ast.FieldList) bool {
	t := recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if g, ok := t.(*ast.IndexExpr); ok {
		t = g.X
	}
	id, ok := t.(*ast.Ident)
	return ok && id.IsExported()
}

// Methods that implement well-known interfaces document themselves.
func isInterfaceMethodName(n string) bool {
	switch n {
	case "Error", "String", "Unwrap", "Is", "ServeHTTP", "WriteHeader", "Write", "Read", "Close", "Execute":
		return true
	}
	return false
}
