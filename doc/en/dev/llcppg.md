llcppg Design
=====

### Type Mapping

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
