# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Core Build Commands
- `go build ./cmd/llcppg` - Build main llcppg command
- `go build ./cmd/gogensig` - Build Go code generator
- `go build ./cmd/llcppcfg` - Build configuration file generator
- `llgo install ./_xtool/llcppsymg` - Install symbol generator (requires LLGo)
- `llgo install ./_xtool/llcppsigfetch` - Install signature fetcher (requires LLGo)

### Testing Commands
- `go test ./...` - Run all tests
- `go test ./cmd/gogensig` - Test Go code generator
- `go test ./cl/internal/convert` - Test conversion logic
- `go test ./config` - Test configuration handling

### Installation Prerequisites
```bash
# Install system dependencies
brew install cjson                    # macOS
apt-get install libcjson-dev         # Linux

# Install LLGo tools (requires LLGo to be installed first)
llgo install ./_xtool/llcppsymg
llgo install ./_xtool/llcppsigfetch

# Install Go tools
go install ./cmd/llcppcfg
go install ./cmd/gogensig
go install .
```

## Architecture Overview

### Core Pipeline
llcppg follows a three-phase conversion pipeline:

1. **Symbol Generation** (`llcppsymg`): Extracts symbols from C/C++ libraries
2. **Signature Extraction** (`llcppsigfetch`): Parses C/C++ headers using clang
3. **Go Code Generation** (`gogensig`): Converts AST to Go bindings

### Key Components

#### Main Tools
- **llcppg** (`cmd/llcppg/`): Main orchestrator and CLI entry point
- **llcppsymg** (`_xtool/llcppsymg/`): Symbol generator (LLGo-compiled)
- **llcppsigfetch** (`_xtool/llcppsigfetch/`): Header parser (LLGo-compiled) 
- **gogensig** (`cmd/gogensig/`): Go code generator
- **llcppcfg** (`cmd/llcppcfg/`): Configuration file generator

#### Core Modules
- **ast/** - AST node definitions for C/C++ parsing
- **cl/** - Core conversion logic from C/C++ to Go
- **config/** - Configuration file handling (`llcppg.cfg`)
- **parser/** - C/C++ header parsing utilities

### File Formats

#### Configuration (`llcppg.cfg`)
```go
type Config struct {
    Name           string            // Package name
    CFlags         string            // Compiler flags  
    Libs           string            // Library flags
    Include        []string          // Header files to convert
    TrimPrefixes   []string          // Prefixes to remove from names
    Deps           []string          // Package dependencies
    SymMap         map[string]string // Symbol name mappings
    TypeMap        map[string]string // Type name mappings
    StaticLib      bool              // Use static library symbols
    HeaderOnly     bool              // Header-only mode
}
```

#### Symbol Table (`llcppg.symb.json`)
Maps C/C++ symbols to Go function names and handles symbol resolution.

#### Public API (`llcppg.pub`)
Simple text format mapping C types to Go types for dependency resolution.

### Conversion Flow
```
C/C++ Headers + Libraries → llcppsymg → llcppg.symb.json
                          ↓
C/C++ Headers → llcppsigfetch → JSON AST → gogensig → Go Bindings
```

## Key Development Areas

### Adding New AST Node Types
- Define in `ast/ast.go`
- Add conversion logic in `cl/nc/nodeconv.go`  
- Update Go generation in `cl/internal/convert/`

### Extending Type Mappings
- Builtin types: `cl/internal/convert/builtin.go`
- Custom mappings: Configure via `typeMap` in `llcppg.cfg`
- Dependency types: Handle in `cl/internal/convert/deps.go`

### Testing Structure
- Unit tests alongside source files (`*_test.go`)
- Integration tests in `_cmptest/` directory
- Test data in `testdata/` subdirectories
- Conversion tests in `cl/internal/convert/_testdata/`

## Common Patterns

### Configuration Loading
```go
cfg, err := config.Load("llcppg.cfg")
```

### AST Processing
The codebase uses a visitor pattern for AST traversal with type-specific handling in the `cl/nc/` package.

### Symbol Resolution
Symbol resolution happens in two phases:
1. Extract from libraries (`llcppsymg`)
2. Match with header declarations (`gogensig`)

## Dependencies and Ecosystem

### External Dependencies
- **LLGo**: Required for compiling `_xtool/` components
- **clang**: Used for C/C++ header parsing
- **Go Plus ecosystem**: Core dependency for Go+ language features

### Standard Library Mapping
The project uses a special `c/` prefix convention for standard library types:
- `c` → `github.com/goplus/lib/c`
- `c/os` → `github.com/goplus/lib/c/os`
- `c/time` → `github.com/goplus/lib/c/time`

## Testing and Validation

### Test Library Generation
Use `_cmptest/` for comprehensive library binding tests:
```bash
cd _cmptest
go test -v ./...
```

### Manual Testing
Generate bindings for test libraries in `_llcppgtest/`:
```bash
cd _llcppgtest/cjson
llcppg llcppg.cfg
```

## Important Notes

- The `_xtool/` components must be compiled with LLGo, not regular Go
- Symbol generation requires matching library files with header declarations
- Header file order in `include` array matters for type resolution
- Use `mix: true` for headers mixed with system headers in same directory