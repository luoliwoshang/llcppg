package transitive

import (
	"testpub/basic"
	"testpub/deps/a"
)

type Holder struct {
	View  *a.View
	Point *basic.Point
	Size  basic.Length
}
