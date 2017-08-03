#!/bin/sh
set -e

USER=root
docker build -t flexvolume-driver-rpm-builder .
echo "Cleaning output directory..."
OUTPUT=/tmp/kube-tmp/
rm -rf $OUTPUT/rpm
mkdir -p $OUTPUT/rpm
docker run --rm -v $OUTPUT/rpm/:/root/rpmbuild/RPMS/ flexvolume-driver-rpm-builder $1

echo
echo "----------------------------------------"
echo
echo "RPMs written to: "
sudo ls $OUTPUT/rpm/*/
echo
echo "Yum repodata written to: "
sudo ls $OUTPUT/rpm/*/repodata/
