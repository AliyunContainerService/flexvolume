FROM registry.aliyuncs.com/acs/alpine:3.3
RUN apk add --update curl && rm -rf /var/cache/apk/*
RUN apk --update add fuse curl libxml2 openssl libstdc++ libgcc && rm -rf /var/cache/apk/*

RUN mkdir -p /acs
COPY nsenter /acs/nsenter
COPY bin/flexvolume /acs/flexvolume
COPY entrypoint.sh /acs/entrypoint.sh
COPY ossfs_1.80.3_centos7.0_x86_64.rpm /acs/ossfs_1.80.3_centos7.0_x86_64.rpm
COPY ossfs_1.80.3_ubuntu14.04_amd64.deb /acs/ossfs_1.80.3_ubuntu14.04_amd64.deb
COPY ossfs_1.80.3_ubuntu16.04_amd64.deb /acs/ossfs_1.80.3_ubuntu16.04_amd64.deb
COPY cpfs-client-1.2.1-centos.x86_64.rpm /acs/cpfs-client-1.2.1-centos.x86_64.rpm
COPY kernel-devel-3.10.0-693.2.2.el7.x86_64.rpm /acs/kernel-devel-3.10.0-693.2.2.el7.x86_64.rpm
COPY kernel-devel-3.10.0-862.14.4.el7.x86_64.rpm /acs/kernel-devel-3.10.0-862.14.4.el7.x86_64.rpm
COPY kernel-devel-3.10.0-957.5.1.el7.x86_64.rpm /acs/kernel-devel-3.10.0-957.5.1.el7.x86_64.rpm

RUN chmod 755 /acs/*

ENTRYPOINT ["/acs/entrypoint.sh"]
