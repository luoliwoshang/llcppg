package typedef_reuse

import "github.com/goplus/lib/c"

type Point struct {
	X c.Int
	Y c.Int
}
type Foo = c.Int
