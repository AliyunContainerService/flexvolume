#!/usr/bin/env bash
set -e

cd ${GOPATH}/src/gitlab.alibaba-inc.com/acs/flexvolume/
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

if [ "$1" == "" ]; then
  cd package
  versioninfo=`cat ${GOPATH}/src/gitlab.alibaba-inc.com/acs/flexvolume/provider/utils/help.go | grep "VERSION = \"" | grep -v "#"`
  version=`echo $versioninfo | awk -F '\"' '{ print $2 }'`

  docker build -t=registry.cn-hangzhou.aliyuncs.com/sigma_test/flexvolume:$version ./
  docker push registry.cn-hangzhou.aliyuncs.com/sigma_test/flexvolume:$version
  cd ..
fi
