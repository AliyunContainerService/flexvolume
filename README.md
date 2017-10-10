# 阿里云盘 flexvolume driver

针对阿里云盘开发的flexvolume 类型的插件，可以支持kubernetes pod 绑定使用阿里云盘。

此版本支持flexvolume,静态pv. 对于动态pv通过provision-controller支持.

## 如何使用该插件：

目前需要kubelet关闭`--enable-controller-attach-detach`选项。在node节点进行如下操作：

```
# git clone https://github.com/AliyunContainerService/flexvolume.git
# cd flexvolume
# make
# make install
```
完成以上操作后重启kubelet。

## 阿里云盘使用实例见example文件夹


## ROADMAP

- 支持动态pv（已实现，尚未开源）
