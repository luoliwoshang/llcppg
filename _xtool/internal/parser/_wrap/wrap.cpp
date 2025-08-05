// NOTE(zzy):temp define in current directory, need to be removed when support libclang at llpkg
#include <clang-c/Index.h>
#include <stdio.h>

extern "C" {
int wrap_clang_isCursorDefinition(CXCursor *cursor) { return clang_isCursorDefinition(*cursor); }
}
