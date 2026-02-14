#!/bin/bash
set -e

# for test
go install -v ./cmd/llcppcfg
echo "llcppcfg is install"
go install -v ./cmd/llcppgtest
echo "llcppgtest is install"

# main process required
llgo install -a ./_xtool/llcppsymg
echo "llcppsymg is install"
llgo install -a ./_xtool/llcppsigfetch
echo "llcppsigfetch is install"
llgo install -a -v ./cmd/gogensig
echo "gogensig is install"
go install -v ./cmd/llcppg
echo "llcppg is install"
