#include <clang-c/Index.h>
#include <stdio.h>

extern "C" {
// void wrap_clang_getCursorReferenced(CXCursor *cur, CXCursor *referenced) {
//     *referenced = clang_getCursorReferenced(*cur);
// }

// void wrap_clang_getCursorDefinition(CXCursor *C, CXCursor *def) { *def = clang_getCursorDefinition(*C); }

// unsigned is_definition = clang_isCursorDefinition(cursor);
int wrap_clang_isCursorDefinition(CXCursor *cursor) { return clang_isCursorDefinition(*cursor); }

// int wrap_clang_Cursor_isNull(CXCursor *cursor) { return clang_Cursor_isNull(*cursor); }
}