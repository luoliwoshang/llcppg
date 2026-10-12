package typedef_reuse

import "github.com/goplus/lib/c"

type Foo = c.Int

type Point struct {
	X c.Int
	Y c.Int
}
