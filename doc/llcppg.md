llcppg Design
=====

## Usage

```sh
llcppg [config-file]
```

If `config-file` is not specified, a `llcppg.cfg` file is used in current directory. The configuration file format is as follows:

```json
{
  "name": "inih",
  "cflags": "$(pkg-config --cflags inireader)",
  "include": [
    "INIReader.h",
    "AnotherHeaderFile.h"
  ],
  "libs": "$(pkg-config --libs inireader)",
  "trimPrefixes": ["Ini", "INI"],
  "cplusplus":true,
  "deps":["c","github.com/..../third"],
  "mix":false 
}
```

## Steps

1. llcppsymg: Generate symbol table for a C/C++ library
2. Manually modify the desired Go symbols in symbol table
3. llcppsigfetch: Fetch information of C/C++ symbols
4. gogensig: Generate a go package by information of symbols


### llcppsymg

```sh
llcppsymg config-file
llcppsymg -  # read config from stdin
```

It generates a symbol table file named `llcppg.symb.json`. Its file format is as follows:

```json
[
  {
    "mangle": "_ZN9INIReaderC1EPKcm",
    "c++": "INIReader::INIReader(char const*, unsigned long)",
    "go": "(*Reader).Init__0"
  }
]
```


### llcppsigfetch

```sh
llcppsigfetch config-file
llcppsigfetch -  # read config from stdin
```

It fetches information of C/C++ symbols and print to stdout. Its format is as follows:

```json
[
  {
    "path": "/path/to/file.h",
    "doc": {
      "decls": [],
      "macros": [],
      "includes": [
        {
          "path": "incfile.h"
        }
      ]
    }
  }
]
```

### gogensig

```sh
gogensig ast-file
gogensig -  # read AST from stdin
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