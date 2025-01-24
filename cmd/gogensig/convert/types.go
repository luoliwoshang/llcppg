package convert

import "github.com/goplus/llcppg/ast"

type Types struct {
	definitions map[string]ast.Node // USR -> Node
}

func NewTypes() *Types {
	return &Types{
		definitions: make(map[string]ast.Node),
	}
}

func (t *Types) Lookup(name string) (ast.Node, bool) {
	decl, ok := t.definitions[name]
	return decl, ok
}

func (t *Types) Register(usr string, node ast.Node) {
	t.definitions[usr] = node
}
