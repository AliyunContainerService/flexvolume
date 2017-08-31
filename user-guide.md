# Alicloud disk driver user guide

## install alicloud disk driver

**获取阿里云盘 driver rpm包**

```shell
wget http://aliacs-k8s.oss.aliyuncs.com/rpm/1.7.2/alicloud-disk-1.7.2-1.0.x86_64.rpm
```

**安装阿里云盘 driver rpm包**

```shell
sudo rpm -i alicloud-disk-1.7.2-1.0.x86_64.rpm
```

**重启kubelet使driver生效**

```shell
sudo systemctl restart kubelet.service
```

## 使用范例

### 直接使用alicloud～disk
Create deployment with spec `deployment.yaml`.（yaml文件见example文件夹） 注意要保证云盘`d-bp1au6l6mbscl64apz0n`已经存在且未被使用，volume name 要与阿里云盘id相同

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: flexvolume-nginx-deployment
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: flexvolume-nginx
        image: nginx
        volumeMounts:
            - name: "d-bp1au6l6mbscl64apz0n"
              mountPath: "/var/lib"
      volumes:
      - name: "d-bp1au6l6mbscl64apz0n"
        flexVolume:
            driver: "alicloud/disk"
            fsType: "ext4"
            options:
                volumeId: "d-bp1au6l6mbscl64apz0n"
```

### 使用PV/PVC（目前不支持动态pv）

- Create pv with spec `pv.yaml`  注意pv name 要与 阿里云盘id相同

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: d-bp1au6l6mbscl64apz0n
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
      volumeId: "d-bp1au6l6mbscl64apz0n" 
```

- Create PVC with spec `pvc.yaml`

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pv-claim
spec:
  storageClassName: slow
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
```

- Create deployment with spec `deploy-pvc.yaml`

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: deployment-nginx-flexvolume-alicloud-pv
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx-flexvolume-alicloud-pv
        image: nginx
        volumeMounts:
            - name: pv-storage
              mountPath: "/var/lib"
      volumes:
      - name: pv-storage
        persistentVolumeClaim:
            claimName: pv-claim
```