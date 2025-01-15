package main

import (
	"github.com/goplus/llcppg/_xtool/llcppsigfetch/parse"
	test "github.com/goplus/llcppg/_xtool/llcppsigfetch/parse/cvt_test"
	"github.com/goplus/llcppg/types"
)

func main() {
	TestForwardDecl()
	TestForwardDeclCrossFile()
}

func TestForwardDecl() {
	test.RunTestWithConfig(&parse.ContextConfig{
		Conf: &types.Config{Cplusplus: false, Include: []string{"hfile/forwarddecl.h"}},
	}, []string{
		"./hfile/forwarddecl.h",
	})
}

func TestForwardDeclCrossFile() {
	test.RunTestWithConfig(&parse.ContextConfig{
		Conf: &types.Config{Cplusplus: false, Include: []string{"hfile/def.h"}},
	}, []string{
		"./hfile/def.h",
	})
}
