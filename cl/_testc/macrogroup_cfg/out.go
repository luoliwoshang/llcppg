package foo

const LLGoPackage = "link: -L/path/foo -lfoo"

// * basic types
const (
	TNONE    = -1
	TNIL     = 0
	TBOOLEAN = 1
	TNUMBER  = 2
	TSTRING  = 3
	NUMTYPES = 4
)

// mixed kinds, user grouped
const (
	CFG_FIRST = 10
	CFG_NAME  = "mix"
	CFG_LAST  = 20
)
