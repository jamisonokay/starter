#!/bin/sh
set -e
mkdir -p tmp
go build -o tmp/main ./cmd
exec ./tmp/main
