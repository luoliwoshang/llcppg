#!/bin/bash
set -e

# for test
go install -v ./cmd/llcppcfg
go install -v ./cmd/llcppgtest

# Optional standalone debug modules:
# The main pipeline is now fully orchestrated by the single `llcppg` binary,
# and built with LLGo, so these are not required for normal installation.
# Keep them here for on-demand debugging or isolated tool usage.
# llgo install ./_xtool/llcppsymg
# llgo install ./_xtool/llcppsigfetch
# llgo install -v ./cmd/gogensig

# main process required
llgo install -v ./cmd/llcppg
