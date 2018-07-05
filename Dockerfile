FROM golang:1.9.7 AS build-env
COPY . /go/src/github.com/AliyunContainerService/flexvolume/
RUN cd /go/src/github.com/AliyunContainerService/flexvolume/ && ./build.sh

FROM alpine:3.7
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories \
    && apk --no-cache add fuse curl libxml2 openssl libstdc++ libgcc
COPY package /acs
COPY --from=build-env /go/src/github.com/AliyunContainerService/flexvolume/flexvolume-linux /acs/flexvolume

RUN chmod 755 /acs/*

ENTRYPOINT ["/acs/entrypoint.sh"]
