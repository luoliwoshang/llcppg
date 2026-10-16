package STLForwardCompat

import "github.com/goplus/lib/c"

const LLGoPackage = "link: -L$(llvm-config --libdir) -lLLVM; -lLLVM"

type Identity struct {
}

// A tag for constructors accepting ranges.
type FromRangeT struct {
}
type RemoveCvref[T any] struct {
}

// TODO: Remove this in favor of std::type_identity<T> once we switch to C++20.
type TypeIdentity[T any] struct {
}
type IdentityIsTransparent = c.Void
