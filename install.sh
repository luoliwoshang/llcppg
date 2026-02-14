#!/bin/bash
set -e

# for test
go install -v ./cmd/llcppcfg
go install -v ./cmd/llcppgtest

# main process required
llgo install -a ./_xtool/llcppsymg
llgo install -a ./_xtool/llcppsigfetch
llgo install -a -v ./cmd/gogensig
llgo install -a -v ./cmd/llcppg
