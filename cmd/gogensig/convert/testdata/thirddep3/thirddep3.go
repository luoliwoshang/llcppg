package thirddep3

import (
	_ "unsafe"

	"github.com/goplus/llgo/c"
)

type ThirdDep3 struct {
	C c.Long
}
