FROM golang:1.9.7 AS build-env
COPY . /go/src/github.com/AliyunContainerService/flexvolume/
RUN cd /go/src/github.com/AliyunContainerService/flexvolume/package && ./build.sh

FROM alpine:3.7
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories \
    && apk --no-cache add fuse curl libxml2 openssl libstdc++ libgcc
RUN mkdir -p /acs
COPY --from=build-env /go/src/github.com/AliyunContainerService/flexvolume/package/bin/flexvolume /acs/flexvolume
COPY package/nsenter /acs/nsenter
COPY package/entrypoint.sh /acs/entrypoint.sh
COPY package/ossfs_1.80.3_centos7.0_x86_64.rpm /acs/ossfs_1.80.3_centos7.0_x86_64.rpm
COPY package/ossfs_1.80.3_ubuntu14.04_amd64.deb /acs/ossfs_1.80.3_ubuntu14.04_amd64.deb
COPY package/ossfs_1.80.3_ubuntu16.04_amd64.deb /acs/ossfs_1.80.3_ubuntu16.04_amd64.deb

RUN chmod 755 /acs/*

ENTRYPOINT ["/acs/entrypoint.sh"]
