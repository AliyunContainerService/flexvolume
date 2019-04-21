#!/bin/sh

# Get System version
host_os="centos-7-4"

/acs/nsenter --mount=/proc/1/ns/mnt which lsb_release
lsb_release_exist=$?
if [ "$lsb_release_exist" != "0" ]; then
  /acs/nsenter --mount=/proc/1/ns/mnt ls /etc/os-release
  os_release_exist=$?
fi

if [ "$lsb_release_exist" = "0" ]; then
    os_info=`/acs/nsenter --mount=/proc/1/ns/mnt lsb_release -a`

    if [ `echo $os_info | grep CentOS | grep 7.2 | wc -l` != "0" ]; then
        host_os="centos-7-2"
    elif [ `echo $os_info | grep CentOS | grep 7.3 | wc -l` != "0" ]; then
        host_os="centos-7-3"
    elif [ `echo $os_info | grep CentOS | grep 7.4 | wc -l` != "0" ]; then
        host_os="centos-7-4"
    elif [ `echo $os_info | grep CentOS | grep 7.5 | wc -l` != "0" ]; then
        host_os="centos-7-5"
    elif [ `echo $os_info | grep CentOS | grep 7. | wc -l` != "0" ]; then
        host_os="centos-7"
    elif [ `echo $os_info | grep 14.04 | wc -l` != "0" ]; then
        host_os="ubuntu-1404"
    elif [ `echo $os_info | grep 16.04 | wc -l` != "0" ]; then
        host_os="ubuntu-1604"
    elif [ `echo $os_info | grep Aliyun | wc -l` != "0" ]; then
        host_os="alios"
    else
        echo "OS is not ubuntu 1604/1404, Centos7"
        echo "system information: "$os_info
        exit 1
    fi

elif [ "$os_release_exist" = "0" ]; then
    osId=`/acs/nsenter --mount=/proc/1/ns/mnt cat /etc/os-release | grep "ID="`
    osVersion=`/acs/nsenter --mount=/proc/1/ns/mnt cat /etc/os-release | grep "VERSION_ID="`

    if [ `echo $osId | grep "centos" | wc -l` != "0" ]; then
        if [ `echo $osVersion | grep "7" | wc -l` = "1" ]; then
          host_os="centos-7"
        fi
    elif [ `echo $osId | grep "alios" | wc -l` != "0" ];then
       if [ `echo $osVersion | grep "7" | wc -l` = "1" ]; then
         host_os="centos-7"
       fi
    elif [ `echo $osId | grep "ubuntu" | wc -l` != "0" ]; then
        if [ `echo $osVersion | grep "14.04" | wc -l` != "0" ]; then
          host_os="ubuntu-1404"
        elif [ `echo $osVersion | grep "16.04" | wc -l` != "0" ]; then
          host_os="ubuntu-1604"
        fi
    fi
fi

restart_kubelet="false"

install_disk() {
    # first install
    if [ ! -f "/host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk" ];then
        mkdir -p /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/
        cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk
        chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk

    # update status
    else
        oldmd5=`md5sum /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk | awk '{print $1}'`
        newmd5=`md5sum /acs/flexvolume | awk '{print $1}'`

        # update disk bianary
        if [ "$oldmd5" != "$newmd5" ]; then
            rm -rf /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk
            cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk
            chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk

        fi
    fi

    # generate disk config for Apsara Stack
    if [ "$ACCESS_KEY_ID" != "" ] && [ "$ACCESS_KEY_SECRET" != "" ] && [ "$ECS_ENDPOINT" != "" ]; then
        mkdir -p /host/etc/.volumeak/
        echo -n $ACCESS_KEY_ID > /host/etc/.volumeak/diskAkId
        echo -n $ACCESS_KEY_SECRET > /host/etc/.volumeak/diskAkSecret
        echo -n $ECS_ENDPOINT > /host/etc/.volumeak/diskEcsEndpoint
    fi

}

install_nas() {
    # install nfs-client
    if [ ! `/acs/nsenter --mount=/proc/1/ns/mnt which mount.nfs4` ]; then
        if [ "$host_os" = "centos-7-4" ] || [ "$host_os" = "centos-7-3" ] || [ "$host_os" = "centos-7-5" ] || [ "$host_os" = "centos-7" ] || [ "$host_os" = "alios" ] ; then
            /acs/nsenter --mount=/proc/1/ns/mnt yum install -y nfs-utils

        elif [ "$host_os" = "ubuntu-1404" ] || [ "$host_os" = "ubuntu-1604" ]; then
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get update -y
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get install -y nfs-common
        fi
    fi

    # install lsof tool
    #if [ ! `/acs/nsenter --mount=/proc/1/ns/mnt which lsof` ]; then
    #    if [ "$host_os" = "centos-7-4" ] || [ "$host_os" = "centos-7-3" ] || [ "$host_os" = "centos-7-5" ] || [ "$host_os" = "centos-7" ]; then
    #        /acs/nsenter --mount=/proc/1/ns/mnt yum install -y lsof
    #    fi
    #fi

    # first install
    if [ ! -f "/host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas" ];then
        mkdir -p /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/
        cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas
        chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas

    # update nas
    else
        oldmd5=`md5sum /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas | awk '{print $1}'`
        newmd5=`md5sum /acs/flexvolume | awk '{print $1}'`

        # install a new bianary
        if [ "$oldmd5" != "$newmd5" ]; then
            rm -rf /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas
            cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas
            chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas
        fi
    fi

}

install_oss() {

    ossfsVer="1.80.3"

    # install OSSFS
    if [ ! `/acs/nsenter --mount=/proc/1/ns/mnt which ossfs` ]; then
        if [ "$host_os" = "centos-7-4" ] || [ "$host_os" = "centos-7-3" ] || [ "$host_os" = "centos-7-5" ] || [ "$host_os" = "centos-7" ] || [ "$host_os" = "alios" ]; then
            cp /acs/ossfs_${ossfsVer}_centos7.0_x86_64.rpm /host/usr/
            /acs/nsenter --mount=/proc/1/ns/mnt yum localinstall -y /usr/ossfs_${ossfsVer}_centos7.0_x86_64.rpm

        elif [ "$host_os" = "ubuntu-1404" ]; then
            cp /acs/ossfs_${ossfsVer}_ubuntu14.04_amd64.deb /host/usr/
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get update -y
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get install -y gdebi-core
            /acs/nsenter --mount=/proc/1/ns/mnt gdebi -n /usr/ossfs_${ossfsVer}_ubuntu14.04_amd64.deb

        elif [ "$host_os" = "ubuntu-1604" ]; then
            cp /acs/ossfs_${ossfsVer}_ubuntu16.04_amd64.deb /host/usr/
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get update -y
            /acs/nsenter --mount=/proc/1/ns/mnt apt-get install -y gdebi-core
            /acs/nsenter --mount=/proc/1/ns/mnt gdebi -n /usr/ossfs_${ossfsVer}_ubuntu16.04_amd64.deb
        fi

    # update OSSFS
    else
        oss_info=`/acs/nsenter --mount=/proc/1/ns/mnt ossfs --version`
        vers_conut=`echo $oss_info | grep ${ossfsVer} | wc -l`
        if [ "$vers_conut" = "0" ]; then
            if [ "$host_os" = "centos-7-4" ] || [ "$host_os" = "centos-7-3" ] || [ "$host_os" = "centos-7-5" ] || [ "$host_os" = "centos-7" ]; then
                /acs/nsenter --mount=/proc/1/ns/mnt yum remove -y ossfs
                cp /acs/ossfs_${ossfsVer}_centos7.0_x86_64.rpm /host/usr/
                /acs/nsenter --mount=/proc/1/ns/mnt yum localinstall -y /usr/ossfs_${ossfsVer}_centos7.0_x86_64.rpm

            elif [ "$host_os" = "ubuntu-1404" ]; then
                /acs/nsenter --mount=/proc/1/ns/mnt apt-get remove -y ossfs
                cp /acs/ossfs_${ossfsVer}_ubuntu14.04_amd64.deb /host/usr/
                /acs/nsenter --mount=/proc/1/ns/mnt gdebi -n /usr/ossfs_${ossfsVer}_ubuntu14.04_amd64.deb

            elif [ "$host_os" = "ubuntu-1604" ]; then
                /acs/nsenter --mount=/proc/1/ns/mnt apt-get remove -y ossfs
                cp /acs/ossfs_${ossfsVer}_ubuntu16.04_amd64.deb /host/usr/
                /acs/nsenter --mount=/proc/1/ns/mnt gdebi -n /usr/ossfs_${ossfsVer}_ubuntu16.04_amd64.deb
            fi
        fi
    fi


    # first install OSS
    if [ ! -f "/host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss" ];then
        mkdir -p /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/
        cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss
        chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss

    # update oss
    else
        oldmd5=`md5sum /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss | awk '{print $1}'`
        newmd5=`md5sum /acs/flexvolume | awk '{print $1}'`

        # install a new bianary
        if [ "$oldmd5" != "$newmd5" ]; then
            rm -rf /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss
            cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss
            chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss

        fi
    fi

    # generate oss ak
    if [ "$ACCESS_KEY_ID" != "" ] && [ "$ACCESS_KEY_SECRET" != "" ]; then
        mkdir -p /host/etc/.volumeak/
        echo -n $ACCESS_KEY_ID > /host/etc/.volumeak/akId
        echo -n $ACCESS_KEY_SECRET > /host/etc/.volumeak/akSecret
    fi

    #if [ -f "/host/etc/.volumeak/akId" ] && [ -f "/host/etc/.volumeak/akSecret" ]; then
    #    mkdir -p /host/etc/.volumeak
    #    if [ -f "/host/etc/.volumeak/akId" ]; then
    #        mv /host/etc/.volumeak/akId /host/etc/.volumeak/akId.bak
    #        mv /host/etc/.volumeak/akSecret /host/etc/.volumeak/akSecret.bak
    #    fi
    #    cp /etc/.volumeak/akId /host/etc/.volumeak/akId
    #    cp /etc/.volumeak/akSecret /host/etc/.volumeak/akSecret
    #else
    #    mkdir -p /host/etc/.volumeak
    #    cp /etc/.volumeak/akId /host/etc/.volumeak/akId
    #    cp /etc/.volumeak/akSecret /host/etc/.volumeak/akSecret
    #fi
}

install_cpfs() {
    # luster version
    lustreversion="2.10.0-1"
    cpfsversion="1.2.1"
    kernelversion74="3.10.0-693.2.2"
    kernelversion75="3.10.0-862.14.4"
    kernelversion76="3.10.0-957.5.1"
    kernelversion=${kernelversion74}

    if [ "$host_os" = "centos-7-4" ] || [ "$host_os" = "centos-7-3" ] || [ "$host_os" = "centos-7-5" ] || [ "$host_os" = "centos-7" ]; then
        # reove lustre-client
        if [ `/acs/nsenter --mount=/proc/1/ns/mnt rpm -qa | grep kmod-lustre-client-$lustreversion | wc -l` != "0" ]; then
            /acs/nsenter --mount=/proc/1/ns/mnt yum remove -y kmod-lustre-client
        fi

        kernelInfo=`/acs/nsenter --mount=/proc/1/ns/mnt uname -a | awk '{print $3}'`

        if [ ${kernelInfo} = ${kernelversion74}".el7.x86_64" ]; then
            kernelversion=${kernelversion74}
        elif [ ${kernelInfo} = ${kernelversion75}".el7.x86_64" ]; then
            kernelversion=${kernelversion75}
        elif [ ${kernelInfo} = ${kernelversion76}".el7.x86_64" ]; then
            kernelversion=${kernelversion76}
        fi

        if [ `/acs/nsenter --mount=/proc/1/ns/mnt rpm -qa | grep kernel-devel-${kernelversion} | wc -l` = "0" ]; then
            cp /acs/kernel-devel-${kernelversion}.el7.x86_64.rpm /host/usr/
            /acs/nsenter --mount=/proc/1/ns/mnt rpm -ivh /usr/kernel-devel-${kernelversion}.el7.x86_64.rpm
        fi

        # install/upgrade cpfs client
        cpfs_num=`/acs/nsenter --mount=/proc/1/ns/mnt rpm -qa | grep cpfs-client- | wc -l`
        cpfsver_num=`/acs/nsenter --mount=/proc/1/ns/mnt rpm -qa | grep cpfs-client-$cpfsversion | wc -l`

        if [ "${cpfsver_num}" = "0" ]; then
            # remove old version
            if [ "${cpfs_num}" != "0" ]; then
                /acs/nsenter --mount=/proc/1/ns/mnt yum remove -y cpfs-client
            fi
            # install cpfs client
            cp /acs/cpfs-client-${cpfsversion}-centos.x86_64.rpm /host/usr/
            /acs/nsenter --mount=/proc/1/ns/mnt yum install -y /usr/cpfs-client-${cpfsversion}-centos.x86_64.rpm
        fi
        if [ ! `/acs/nsenter --mount=/proc/1/ns/mnt which mount.lustre` ]; then
            /acs/nsenter --mount=/proc/1/ns/mnt service cpfs-client rebuild
        fi
    fi

    # first install cpfs
    if [ ! -f "/host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs" ];then
        mkdir -p /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/
        cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs
        chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs

    # upgrade cpfs
    else
        oldmd5=`md5sum /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs | awk '{print $1}'`
        newmd5=`md5sum /acs/flexvolume | awk '{print $1}'`

        # install a new bianary
        if [ "$oldmd5" != "$newmd5" ]; then
            rm -rf /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs
            cp /acs/flexvolume /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs
            chmod 755 /host/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~cpfs/cpfs

        fi
    fi
}

# if kubelet not disable controller, exit
enableADController="false"
count=`ps -ef | grep kubelet | grep "enable-controller-attach-detach=false" | grep -v "grep" | wc -l`
if [ "$count" = "0" ]; then
  configInFile=`/acs/nsenter --mount=/proc/1/ns/mnt cat /var/lib/kubelet/config.yaml | grep enableControllerAttachDetach | grep false | grep -v grep | wc -l`
  if [ "$configInFile" = "0" ]; then
    enableADController=true
  fi
fi

if [ "$enableADController" = "true" ]; then
  echo "kubelet not running in: enable-controller-attach-detach=false, mount maybe failed"
fi

# install plugins
if [ "$ACS_DISK" = "true" ]; then
  install_disk
fi
if [ "$ACS_OSS" = "true" ]; then
  install_oss
fi
if [ "$ACS_NAS" = "true" ]; then
  install_nas
fi

if [ "$ACS_CPFS" = "true" ]; then
  install_cpfs
fi


## monitoring should be here
/acs/flexvolume monitoring
