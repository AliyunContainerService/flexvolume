#!/usr/bin/env bash
set -e

cd flexvolume/
GIT_SHA=`git rev-parse --short HEAD || echo "HEAD"`


export GOARCH="amd64"
export GOOS="linux"
if [[ "$(uname -s)" == "Linux" ]];then
	CGO_ENABLED=1 go build -tags 'netgo' --ldflags '-extldflags "-static"' -o flexvolume-linux 
else
	CGO_ENABLED=0 go build -o flexvolume-linux 
fi

mkdir -p package/bin
mv flexvolume-linux package/bin/flexvolume


docker build -t=flexvolume:version ./
