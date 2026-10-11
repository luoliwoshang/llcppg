package a

import "testpub/basic"

type View struct {
	Record    *basic.Record
	Point     basic.Point
	PointRef  *basic.Point
	Size      basic.Length
	Status    basic.Status
	Value     basic.Value
	Handle    *basic.Handle
	HandlePtr basic.HandlePtr
	Callback  basic.Callback
}
type Size = basic.Length
