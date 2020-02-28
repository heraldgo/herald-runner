#!/bin/sh

cd $(dirname "$0")

build_dir=herald-runner

version=$(grep 'const Version' ../version.go | cut '-d"' -f2)

./build.sh

./upload.py "$version" $build_dir/*.tar.gz
