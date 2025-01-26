package convert

import "github.com/goplus/llcppg/ast"

type Types struct {
	Defs map[string]ast.Node // USR -> Node
	Deps map[string][]string // USR -> []USR
}

func NewTypes() *Types {
	return &Types{
		Defs: make(map[string]ast.Node),
		Deps: make(map[string][]string),
	}
}

func (t *Types) Lookup(name string) (ast.Node, bool) {
	decl, ok := t.Defs[name]
	return decl, ok
}

func (t *Types) Register(usr string, node ast.Node) {
	t.Defs[usr] = node
}

func (t *Types) RecordDep(usr string, depUsr string) {
	t.Deps[usr] = append(t.Deps[usr], depUsr)
}

func (t *Types) LookupDeps(usr string) []string {
	return t.Deps[usr]
}
