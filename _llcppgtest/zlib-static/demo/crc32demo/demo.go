package main

import (
	"fmt"
	"unsafe"
	"zlib-static"
)

func main() {
	ul := zlib_static.ULong(0)
	data := "Hello world"
	res := ul.Crc32Z(
		(*zlib_static.Bytef)(unsafe.Pointer(unsafe.StringData(data))),
		zlib_static.ZSizeT(uintptr(len(data))),
	)
	fmt.Printf("%08x\n", res)
}
