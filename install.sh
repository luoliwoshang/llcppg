#!/bin/bash
set -e

# for test
go install -v ./cmd/llcppcfg
go install -v ./cmd/llcppgtest

# main process required
llgo install -v ./_xtool/llcppsymg
llgo install -v ./_xtool/llcppsigfetch
go install -v ./cmd/gogensig
go install -v ./cmd/llcppg
