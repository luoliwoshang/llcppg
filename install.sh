#!/bin/bash
set -e

# for test
go install -v ./cmd/llcppcfg
go install -v ./cmd/llcppgtest

# Main pipeline does not depend on the tools below.
# They are mainly installed for debugging and standalone troubleshooting.
llgo install ./_xtool/llcppsymg
llgo install ./_xtool/llcppsigfetch
llgo install -v ./cmd/gogensig

# main process required
llgo install -v ./cmd/llcppg
