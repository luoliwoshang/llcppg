llcppg Design
=====

### Type Mapping

#### Basic Type Mapping

All basic types are imported from `github.com/goplus/lib/c` and mapped accordingly.

| C Type | Go Type |
|--------|---------|
| void | c.Void |
| bool | bool |
| char | c.Char |
| wchar_t | int16 |
| char16_t | int16 |
| char32_t | int32 |
| short | int16 |
| unsigned short | uint16 |
| int | c.Int |
| unsigned int | c.Uint |
| long | c.Long |
| unsigned long | c.Ulong |
| long long | c.LongLong |
| unsigned long long | c.UlongLong |
| float | c.Float |
| double | c.Double |
| float complex | complex64 |
| double complex | complex128 |

#### Special Case

##### void *

The pointer type `void*` is mapped to `c.Pointer`.

```c
void *(luaL_testudata) (lua_State *L, int ud, const char *tname);
```
```go
//go:linkname Testudata C.luaL_testudata
func LuaLTestudata(L *State, ud c.Int, tname *c.Char) c.Pointer
```

##### Function pointer

C function pointer types are converted to Go function types with corresponding parameter and return type mappings,And llgo need to add `llgo:type C` tag to the function type.


```c
typedef int (*CallBack)(void *L);
```
```go
// llgo:type C
type CallBack func(c.Pointer) c.Int
```

For function pointer types referenced in function signatures, the type is replaced with the converted Go function type.

```c
void exec(void *L, CallBack cb);
```
```go
// llgo:type C
func Exec(L c.Pointer, cb CallBack)
```

For cases where a parameter in a function signature is an anonymous function pointer (meaning it does not reference a pre-defined function pointer type), it is mapped to the corresponding Go function type.

```c
int sqlite3_exec(
  sqlite3*,                                  /* An open database */
  const char *sql,                           /* SQL to be evaluated */
  int (*callback)(void*,int,char**,char**),  /* Callback function */
  void *,                                    /* 1st argument to callback */
  char **errmsg                              /* Error msg written here */
);
```
```go
// llgo:link (*Sqlite3).Exec C.sqlite3_exec
func (recv_ *Sqlite3) Exec(sql *c.Char, callback func(c.Pointer, c.Int, **c.Char, **c.Char) c.Int, __llgo_arg_2 c.Pointer, errmsg **c.Char) c.Int {
	return 0
}
```

For struct fields that are named function pointer types, the field type is replaced with a `c.Pointer` for description.

```c
typedef struct Stream {
    CallBack cb;
} Stream;
```
```go
type Stream struct {
	Cb c.Pointer
}
```
Due to the characteristics of LLGo, an anonymous function type cannot be directly declared as a Field Type. So to preserve as much information as possible, we need to use a `c.Pointer` to describe the field type.

For anonymous function pointer types, llgo will build a public function type for them and reference it,and ensure that the anonymous type is unique, the naming rule for the corresponding type will be:
```
LLGO_<namespaces>_<typename>_<nested_field_typename>_<fieldname>
```

```c
typedef struct Hooks {
    void *(*malloc_fn)(size_t sz);
    void (*free_fn)(void *ptr);
} Hooks;
```
```go
// llgo:type C
type LLGO_Hooks_MallocFn func(c.SizeT) c.Pointer
// llgo:type C
type LLGO_Hooks_FreeFn func(c.Pointer)

type Hooks struct {
	MallocFn LLGO_Hooks_MallocFn
	FreeFn   LLGO_Hooks_FreeFn
}
```

with namespace

```c
namespace A {
  struct Hooks {
      void *(*malloc_fn)(size_t sz);
      void (*free_fn)(void *ptr);
  } Hooks;
}
```
```go
// llgo:type C
type LLGO_A_Hooks_MallocFn func(c.SizeT) c.Pointer
// llgo:type C
type LLGO_A_Hooks_FreeFn func(c.Pointer)

type Hooks struct {
	MallocFn LLGO_A_Hooks_MallocFn
	FreeFn   LLGO_A_Hooks_FreeFn
}
```
with nested struct's function pointer
```c
struct Foo {
    struct {
        void *(*malloc_fn)(size_t sz);
        void (*free_fn)(void *ptr);
    } Hooks;
};
```
```go
// llgo:type C
type LLGO_Foo_Hooks_MallocFn func(c.SizeT) c.Pointer

// llgo:type C
type LLGO_Foo_Hooks_FreeFn func(c.Pointer)

type Foo struct {
	Hooks struct {
		MallocFn LLGO_Foo_Hooks_MallocFn
		FreeFn   LLGO_Foo_Hooks_FreeFn
	}
}
```

##### Array

Arrays in C are mapped differently depending on their context - function parameters versus struct fields.

###### As Function Param

Arrays in function parameters are converted to pointers.

```c
void foo(unsigned int a[], double b[3]);
```
```go
//go:linkname Foo C.foo
func Foo(a *c.Uint, b *c.Double)
```

###### As Struct Field

Arrays in struct fields maintain their fixed-length array form to preserve memory layout compatibility with the original C struct.

```c
typedef struct Foo {  
    char a[4];  
    int b[3][4];  
} Foo;
```
```go
type Foo struct {
	A [4]c.Char
	B [3][4]c.Int
}
```
###### Multi-dimensional

Multi-dimensional arrays are supported in both contexts, with the same conversion rules applying:

```c
char matrix[3][4];  // In function parameter becomes **c.Char  
char field[3][4];   // In struct field becomes [3][4]c.Char
```


#### Name Mapping Rules

The llcppg system converts C/C++ type names to Go-compatible identifiers following specific transformation rules. These rules ensure generated Go code follows Go naming conventions while maintaining clarity and avoiding conflicts.

##### Public Name Processing
Names starting with underscore or digit are prefixed with "X" to create valid Go identifiers.

##### Type Name Conversion (struct, union, typedef, enum)

1. Remove configured prefixes from `trimPrefixes`
2. Convert to PascalCase if the name starts with a letter
3. If the name starts with an underscore, apply Public Name Processing and preserve the original case after underscores, then convert to PascalCase format

Examples without `trimPrefixes` sconfiguration:
* C: `cJSON_Hooks` → Go: `CJSONHooks`
* C: `xmlAttrHashBucket` → Go: `XmlAttrHashBucket`
* C: `sqlite3_destructor_type` → Go: `Sqlite3DestructorType`

Examples with `trimPrefixes: ["cJSON_", "sqlite3_", "xml"]`:

* C: `cJSON_Hooks` → Go: `Hooks`
* C: `sqlite3_destructor_type` → Go: `DestructorType`
* C: `xmlAttrHashBucket` → Go: `AttrHashBucket`

Examples which is start with underscore:

* C: `_gmp_err` → Go: `X_gmpErr`


##### Macro and Enum Special Rules
For macros and enums after prefix removal:

Letter-starting names: Capitalize first letter only, preserve original format
Underscore/digit-starting names: Apply public name processing,preserve original format

##### Custom Type Mappings

Types with explicit mappings in typeMap configuration bypass all other processing rules:
```json
{  
  "typeMap": {  
    "cJSON": "JSON"  
  }  
}
```
Example: C: `cJSON` → Go: `JSON`

##### Field Name Conversion

Field names must be exportable (public) in Go to allow external access. The conversion rules:

1. Letter-starting fields: Convert to PascalCase
2. Underscore/digit-starting fields: Apply public processing, then convert to PascalCase while preserving case after underscores

##### Param Name Conversion

Parameter names are preserved in their original style without conversion, with only the following special cases being handled:

1. When a parameter conflicts with a keyword, a `_` suffix is added to the parameter name

```c
void(lua_sethook)(lua_State *L, lua_Hook func, int mask, int count);
```
```go
//go:linkname Sethook C.lua_sethook
func LuaSethook(L *LuaState, func_ LuaHook, mask c.Int, count c.Int)
```

2. For variadic parameters, the parameter name is `__llgo_va_list`

```c
LUA_API int(lua_gc)(lua_State *L, int what, ...);
```
```go
//go:linkname Gc C.lua_gc
func LuaGc(L *State, what c.Int, __llgo_va_list ...interface{}) c.Int
```

3. For function signatures where all parameters have no names, the corresponding function signature will not generate parameter names.

4. Once there are named parameters in the function signature, according to Go's rules, all parameter names must be generated in the corresponding Go signature.

C allows mixing named and unnamed parameters in function signatures. For this case, the rule is to generate parameter names like `__llgo_arg_N` for unnamed parameters based on their index in the parameter list.

```c
int OSSL_PROVIDER_add_builtin(OSSL_LIB_CTX *, const char *name);
```
```go
//go:linkname ProviderAddBuiltin C.OSSL_PROVIDER_add_builtin
func OSSLProviderAddBuiltin(__llgo_arg_0 *OSSLLIBCTX, name *c.Char) c.Int
```

And for cases where only variadic parameters appear, llgo requires ` __llgo_va_list ...interface{}` to describe variadic parameters, and the same placeholder name generation processing is needed for this case.

```c
char *mprintf(const char*,...);
```
```go
//go:linkname Mprintf C.mprintf
func Mprintf(__llgo_arg_0 *c.Char, __llgo_va_list ...interface{}) *c.Char
```


### File Generation Rules

#### Generated File Types
* Interface header files: Each header file generates a corresponding .go file
* Implementation files: All generated in a single libname_autogen.go file
* Third-party header files: Skip generation,only as a dependency

#### Header File Concepts
In the `llcppg.cfg`, the `include` field specifies the list of interface header files to be converted. These header files are the primary source for generating Go code, and each listed header file will generate a corresponding .go file.

```json
{
  "name": "xslt",
  "cflags": "$(pkg-config --cflags libxslt)",
  "include": [
    "libxslt/xslt.h",
    "libxslt/security.h"
  ]
}
```
##### Package Header File Determination

llcppg determines whether a header file belongs to the current package based on the following rules:

1. **Interface header files**: Header files explicitly listed in the `include` field
2. **Implementation header files**: Other header files in the same root directory as interface header files
3. **Third-party header files**: Header files that don't belong to the current package (such as standard libraries or third-party dependencies) won't be directly converted but are handled through dependency relationships.

For example, if the configuration includes `libxslt/xslt.h`, and this file contains `#include "xsltexports.h"`, then:
- `xslt.h` is an interface header file, which will generate `xslt.go`
- `xsltexports.h` is an implementation header file, whose content will be generated into `xslt_autogen.go`

###### Example Explanation

For example, the header file paths obtained after linking with Clang in the above example:
```
/opt/homebrew/Cellar/libxslt/1.1.42_1/include/libxslt/xslt.h
/opt/homebrew/Cellar/libxslt/1.1.42_1/include/libxslt/security.h
/opt/homebrew/Cellar/libxslt/1.1.42_1/include/libexslt/exsltconfig.h
```
The calculated common root directory is:
```
/opt/homebrew/Cellar/libxslt/1.1.42_1/include/
```
In `libxslt/xslt.h`, the following header files are referenced:
```c
#include <libxml/tree.h>
#include "xsltexports.h"
```
The corresponding paths are:
`libxml/tree.h` -> `/opt/homebrew/Cellar/libxml2/2.13.5/include/libxml2/libxml/tree.h` (third-party dependency)
`xsltexports.h` -> `/opt/homebrew/Cellar/libxslt/1.1.42_1/include/libxslt/xsltexports.h` (package implementation file)
Since `xsltexports.h` is in the same directory as `libxslt/xslt.h`, it's considered a package implementation file, and its content is generated in `xslt_autogen.go`. Meanwhile, `libxml/tree.h` is not in the same directory and is considered a third-party dependency.

#### Special Case: Mixed Header Files
For cases where package header files are mixed with other header files in the same directory (such as system headers or third-party libraries), you can handle this by setting `mix: true`:

```json
{
  "mix": true
}
```

In this case, only header files explicitly declared in the `include` field are considered package header files, and all others are treated as third-party header files. Note that in this mode, implementation header files of the package also need to be explicitly declared in `include`, otherwise they will be treated as third-party header files and won't be processed. 

This is particularly useful in scenarios like Linux systems where library headers might be installed in common directories (e.g., `/usr/include/sqlite3.h` alongside system headers like `/usr/include/stdio.h`).

### Dependency
llcppg does not convert header files outside of the current package, including any referenced third-party or standard library headers. Instead, it manages cross-package type references and ensures conversion consistency through the `deps` declaration in `llcppg.cfg`, which must include standard library types as well.
```json
{
  "deps":["c/os","github.com/author/pkg"]
}
```

#### Dependency Package Structure
Each dependency package follows a unified file organization structure (using xml2 as an example):
* Converted Go source files
1. HTMLtree.go (generated from HTMLtree.h)
2. HTMLparser.go (generated from HTMLparser.h)
* Configuration files
1. llcppg.cfg (dependency information)
2. llcppg.pub (type mapping information)

##### TypeMapping Examples (llcppg.pub)
Standard Library Type Mapping
`github.com/goplus/lib/c/llcppg.pub`
```
size_t SizeT
intptr_t IntptrT
FILE
```
XML2 Type Mapping From Expamle
`github.com/goplus/..../xml2/llcppg.pub`
```
xml_doc XmlDoc
```

#### Dependency Handling Logic
1. llcppg scans each dependency package's `llcppg.pub` file to obtain type mappings.
2. If the dependency package's `llcppg.cfg` also contains deps configuration, llcppg will recursively process these dependencies.
3. Type mappings from all dependency packages are loaded and registered into the conversion project.
When a header file in the current project references types from third-party packages, it directly searches within the current conversion project scope
 * If a mapped type is found, it is referenced;
 * Otherwise, the user is notified of the missing type and its source header file for conversion.

#### Special Dependency Aliases
In llcppg, there is a consistent pattern for naming aliases related to the standard library. Any alias that starts with `c/` corresponds to a remote repository in the github.com/goplus/llgo.

For example:
* The alias `c` → `github.com/goplus/lib/c`
* The alias `c/os` → `github.com/goplus/lib/c/os`
* The alias `c/time` → `github.com/goplus/lib/c/time`

> Note: Standard library type conversion in llgo is not comprehensive. For standard library types that cannot be found in llgo, you will need to supplement these types in the corresponding package at https://github.com/goplus/llgo.

#### Example
You can specify dependent package paths in the `deps` field of `llcppg.cfg` . For example, in the `_llcppgtest/libxslt` example, since libxslt depends on libxml2, its configuration file looks like this:
```json
{
  "name": "libxslt",
  "cflags": "$(pkg-config --cflags libxslt)",
  "libs": "$(pkg-config --libs libxslt)",
  "trimPrefixes": ["xslt"],
  "deps": ["c/os","github.com/goplus/llpkg/libxml2"],
  "includes":["libxslt/xsltutils.h","libxslt/templates.h"]
}
```

In `libxslt/xsltutils.h`, there are dependencies on `libxml2`'s `xmlChar` and `xmlNodePtr`:
```c
#include <libxml/dict.h>
#include <libxml/xmlerror.h>
#include <libxml/xpath.h>
xmlChar * xsltGetNsProp(xmlNodePtr node, const xmlChar *name, const xmlChar *nameSpace);
```
If `xmlChar` and `xmlNodePtr` mappings are not found (not declare `llcppg-libxml` in `deps`), llcppg will notify the user of these missing types and indicate they are from `libxml2` header files.
The corresponding notification would be:
```bash
convert /path/to/include/libxml2/libxml/xmlstring.h first, declare its converted package in llcppg.cfg deps for load [xmlChar].
convert /path/to/libxml2/libxml/tree.h first, declare its converted package in llcppg.cfg deps for load [xmlNodePtr].
```

For this project, `llcppg` will automatically handle type references to libxml2. During the process, `llcppg` uses the `llcppg.pub` file from the generated libxml2 package to ensure type consistency.
You can see this in the generated code, where libxslt correctly references libxml2's types:
```go
package libxslt

import (
	"github.com/goplus/lib/c"
	"github.com/goplus/llpkg/libxml2"
	"unsafe"
)

/*
 * Our own version of namespaced attributes lookup.
 */
//go:linkname GetNsProp C.xsltGetNsProp
func GetNsProp(node libxml2.NodePtr, name *libxml2.Char, nameSpace *libxml2.Char) *libxml2.Char
```

### Cross-Platform Difference Handling
Use impl.files configuration to handle platform differences:
```json
{
    "impl": [
        {
            "files": ["t1.h", "t2.h"],
            "cond": {
                "os": ["macos", "linux"],
                "arch": ["arm64", "amd64"]
            }
        }
    ]
}
```
The generated t1.go & t2.go files will have platform-specific build tags at the beginning:
macos arm64 t1_macos_arm64.go  t2_macos_arm64.go
```go
// +build macos,arm64
package xxx
```
linux arm64 `t1_linux_arm64.go`  `t2_linux_arm64.go`
```go
// +build linux,arm64
package xxx
```
macos amd64  `t1_macos_amd64.go`  `t2_macos_amd64.go`
```go
// +build macos,amd64
package xxx
```
linux amd64 `t1_linux_amd64.go`  `t2_linux_amd64.go`
```go
// +build linux,amd64
package xxx
```