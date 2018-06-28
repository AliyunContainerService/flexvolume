# 云盘、NAS、OSS存储使用示例

## 阿里云云盘使用指南

### 使用说明

> 1. 云盘为非共享存储，只能同时被一个pod挂载；
> 2. 使用云盘数据卷前需要先申请一个云盘，并获得磁盘ID；
> 3. volumeId: 表示所挂载云盘的磁盘ID；volumeName、PV Name要与之相同；
> 4. 集群中只有同云盘在同一个可用区（Zone）的节点才可以挂载云盘；

### 直接通过 Volume 使用 (replicas = 1)
- Create Pod with spec `disk-deploy.yaml`. 

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-disk-deploy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx-flexvolume-disk
        image: nginx
        volumeMounts:
          - name: "d-bp1j17ifxfasvts3tf40"
            mountPath: "/data"
      volumes:
        - name: "d-bp1j17ifxfasvts3tf40"
          flexVolume:
            driver: "alicloud/disk"
            fsType: "ext4"
            options:
              volumeId: "d-bp1j17ifxfasvts3tf40"
```

### 通过 PV/PVC 使用（目前不支持动态pv）

- Create pv with spec `disk-pv.yaml`. 注意pv name 要与 volumeId相同

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: d-bp1j17ifxfasvts3tf40
spec:
  capacity:
    storage: 20Gi
  accessModes:
    - ReadWriteOnce
  storageClassName: slow
  flexVolume:
    driver: "alicloud/disk"
    fsType: "ext4"
    options:
      volumeId: "d-bp1j17ifxfasvts3tf40" 
```

- Create PVC with spec `disk-pvc.yaml`

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-disk
spec:
  storageClassName: slow
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
```

- Create Pod with spec `disk-pod.yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "flexvolume-alicloud-example"
spec:
  containers:
    - name: "nginx"
      image: "nginx"
      volumeMounts:
          - name: pvc-disk
            mountPath: "/data"
  volumes:
  - name: pvc-disk
    persistentVolumeClaim:
        claimName: pvc-disk
```


## 阿里云NAS使用指南

### 使用说明

NAS为共享存储，可以同时为多个Pod提供共享存储服务；

> 1. server：为NAS数据盘的挂载点，注意区分专有网络、经典网络；详见：[NAS使用](https://help.aliyun.com/document_detail/27531.html?spm=5176.doc60431.6.557.6em3JE)
> 2. path：为NAS数据盘的挂载路径，支持挂载nas子目录；且当子目录不存在时，自动创建子目录并挂载；
> 3. vers：定义nfs挂载协议的版本号，支持：4.0；
> 4. mode：定义挂载目录的访问权限，注意：挂载NAS盘根目录时不能配置挂载权限；

### 使用前准备
> 1. 使用NAS数据卷前需要到NAS控制台手动创建一个NAS数据盘；[NAS使用](https://help.aliyun.com/document_detail/27531.html?spm=5176.doc60431.6.557.6em3JE)
> 2. 需要为NAS数据盘添加挂载点，挂载点类型为VPC，且VPC网络、交换机配置与K8S集群配置相同；权限组选择默认权限组；

### 通过 Volume 方式使用

- Create Pod with spec `nas-deploy.yaml`

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-nas-deploy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx-flexvolume-nas
        image: nginx
        volumeMounts:
          - name: "nas1"
            mountPath: "/data"
      volumes:
        - name: "nas1"
          flexVolume:
            driver: "alicloud/nas"
            options:
              server: "0cd8b4a576-uih75.cn-hangzhou.nas.aliyuncs.com"
              path: "/k8s"
              vers: "4.0"
              mode: "755"
```

## 使用 PV/PVC（目前不支持动态pv）

- Create pv with spec `nas-pv.yaml` 

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-nas
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  storageClassName: fast
  flexVolume:
    driver: "alicloud/nas"
    options:
      server: "0cd8b4a576-uih75.cn-hangzhou.nas.aliyuncs.com"
      path: "/k8s"
      vers: "4.0"
      mode: "755"
```

- Create PVC with spec `nas-pvc.yaml`

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-nas
spec:
  storageClassName: fast
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
```

- Create Pod with spec `nas-pod.yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "flexvolume-nas-example"
spec:
  containers:
    - name: "nginx"
      image: "nginx"
      volumeMounts:
          - name: pvc-nas
            mountPath: "/data"
  volumes:
  - name: pvc-nas
    persistentVolumeClaim:
        claimName: pvc-nas
```


## 阿里云OSS使用指南

### 使用说明

OSS为共享存储，可以同时为多个Pod提供共享存储服务；

> 1. bucket：目前只支持挂载Bucket，不支持挂载Bucket下面的子目录或文件；
> 2. url: OSS endpoint，挂载oss的接入域名；详见：[oss使用](https://help.aliyun.com/document_detail/31837.html?spm=5176.doc31834.2.4.7UIDO1)    
>3. otherOpts: 挂载oss时支持定制化参数输入，格式为: -o *** -o ***；详见：[链接](https://help.aliyun.com/document_detail/32197.html?spm=5176.product31815.6.1044.MLGXff)

注意：使用oss数据卷必须在部署flexvolume服务的时候创建Secret，并输入AK信息；

### 直接使用 Volume 方式

- Create Pod with spec `oss-deploy.yaml`

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-oss-deploy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx-flexvolume-oss
        image: nginx
        volumeMounts:
          - name: "oss1"
            mountPath: "/data"
      volumes:
        - name: "oss1"
          flexVolume:
            driver: "alicloud/oss"
            options:
              bucket: "docker"
              url: "oss-cn-hangzhou.aliyuncs.com"
              otherOpts: "-o max_stat_cache_size=0 -o allow_other"
```

## 使用 PV/PVC（目前不支持动态pv）

- Create pv with spec `oss-pv.yaml` 

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-oss
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  storageClassName: slow
  flexVolume:
    driver: "alicloud/oss"
    options:
      bucket: "docker"
      url: "oss-cn-hangzhou.aliyuncs.com"
      otherOpts: "-o max_stat_cache_size=0 -o allow_other"
```

- Create PVC with spec `oss-pvc.yaml`

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-oss
spec:
  storageClassName: slow
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
```

- Create deployment with spec `oss-pod.yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "flexvolume-oss-example"
spec:
  containers:
    - name: "nginx"
      image: "nginx"
      volumeMounts:
          - name: pvc-oss
            mountPath: "/data"
  volumes:
  - name: pvc-oss
    persistentVolumeClaim:
        claimName: pvc-oss
```