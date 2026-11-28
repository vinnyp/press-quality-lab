package scorecard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

func writerPolicy(source []byte) error {
	f, e := parser.ParseFile(token.NewFileSet(), "scorecard.go", source, 0)
	if e != nil {
		return e
	}
	// All calls in this small renderer are enumerated. A new writer path or
	// alias must be reviewed explicitly, and cannot silently bypass line.
	allowed := map[string]bool{"io.WriteString": true, "strictjson.AppendValue": true, "strings.TrimSuffix": true, "fmt.Sprintf": true, "p.escape": true, "p.line": true, "obj": true, "arr": true, "text": true, "reason": true, "label": true, "len": true, "string": true, "action": true, "listed": true, "showCheck": true}
	var problem error
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch x := call.Fun.(type) {
			case *ast.Ident:
				name = x.Name
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					name = id.Name + "." + x.Sel.Name
				}
			}
			// The alias type conversions have no I/O behavior.
			if name == "" || !allowed[name] {
				problem = fmt.Errorf("unreviewed call %s", name)
			}
			if name == "io.WriteString" {
				if fn.Name.Name != "line" || len(call.Args) != 2 {
					problem = fmt.Errorf("writer outside escaping helper")
				}
				if len(call.Args) == 2 {
					plus, ok := call.Args[1].(*ast.BinaryExpr)
					if !ok {
						problem = fmt.Errorf("unescaped write")
					} else {
						escape, ok := plus.X.(*ast.CallExpr)
						if !ok {
							problem = fmt.Errorf("unescaped write")
						} else if sel, ok := escape.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "escape" {
							problem = fmt.Errorf("unescaped write")
						}
					}
				}
			}
			return true
		})
	}
	return problem
}
func TestWriterPolicy(t *testing.T) {
	// QE14, KTD16: every writer call passes the single escaping helper.
	source, e := os.ReadFile("scorecard.go")
	if e != nil {
		t.Fatal(e)
	}
	if e := writerPolicy(source); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{`func bypass(w io.Writer) { io.WriteString(w,"raw") }`, `func bypass(w io.Writer) { fmt.Fprint(w,"raw") }`} {
		planted := append(append([]byte{}, source...), []byte("\n"+mutation)...)
		if e := writerPolicy(planted); e == nil {
			t.Fatal("writer bypass accepted")
		}
	}
}
