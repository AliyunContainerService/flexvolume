package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	f "gitlab.alibaba-inc.com/acs/flexvolume"
)

const credPath string = "/etc/kubernetes/cloud-config"
const KUBERNETES_ALICLOUD_DISK_DRIVER = "alicloud_disk"

type AlicloudOptions struct {
	FsType   string `json:"kubernetes.io/fsType"`
	VolumeId string `json:"volumeId"`
}

type AlicloudPlugin struct{}

func (AlicloudPlugin) NewOptions() interface{} {
	return &AlicloudOptions{}
}

func (AlicloudPlugin) Init() f.Result {
	return f.Succeed()
}

func (AlicloudPlugin) Attach(opts interface{}, nodeName string) f.Result {
	regionId, instanceId, err := getRegionIdAndInstanceId(nodeName)
	if err != nil {
		return f.Fail(err.Error())
	}

	opt := opts.(*AlicloudOptions)

	raw, err := ioutil.ReadFile(credPath)
	if err != nil {
		return f.Fail("read cred file Failed: ", err.Error())
	}
	var defaultOpt f.DefaultOptions
	err = json.Unmarshal(raw, &defaultOpt)
	if err != nil {
		return f.Fail(err.Error())
	}

	client := NewSDKClient(defaultOpt.Global.AccessKeyID, defaultOpt.Global.AccessKeySecret, instanceId)
	if client == nil {
		return f.Fail("Could not create client")
	}

	attachRequest := &ecs.AttachDiskArgs{
		InstanceId: instanceId,
		DiskId:     opt.VolumeId,
	}

	var devicePath string

	for {
		describeDisksRequest := &ecs.DescribeDisksArgs{
			DiskIds:  []string{opt.VolumeId},
			RegionId: common.Region(regionId),
		}

		// call detach to ensure work after node reboot
		disks, _, err := client.DescribeDisks(describeDisksRequest)
		if err != nil {
			return f.Fail("can not get volume \"", opt.VolumeId, "\":", err.Error())
		}

		if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusInUse {
			/*
				if disks[0].InstanceId != instanceId {
					return f.Fail("disk:", disks[0], "is using by other node: ", disks[0].InstanceId)
				}
			*/
			err = client.DetachDisk(disks[0].InstanceId, disks[0].DiskId)
			if err != nil {
				return f.Fail("Failed to detach: ", err.Error())
			}
		}

		// wait for Detach
		var retryDetachCount = 5
		for {
			retryDetachCount--
			if retryDetachCount < 0 {
				return f.Fail("Describe disk failed: timeout")
			}
			time.Sleep(100 * time.Millisecond)
			disks, _, err := client.DescribeDisks(describeDisksRequest)
			if err != nil {
				return f.Fail("Could not get volume \"", opt.VolumeId, "\": ", err.Error())
			}
			if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusAvailable {
				break
			}
		}

		// list device before attach disk
		var before []string
		files, _ := ioutil.ReadDir("/dev")
		for _, file := range files {
			if !file.IsDir() && strings.Contains(file.Name(), "vd") {
				before = append(before, file.Name())
			}
		}

		if err = client.AttachDisk(attachRequest); err != nil {
			return f.Fail("attach failed:", instanceId, opt.VolumeId)
		}

		// wait for attach
		var retryAttachCount = 5
		for {
			retryAttachCount--
			if retryAttachCount < 0 {
				return f.Fail("Attach disk failed: timeout")
			}
			time.Sleep(100 * time.Millisecond)
			disks, _, err := client.DescribeDisks(describeDisksRequest)
			if err != nil {
				return f.Fail("Could not get volume \"", opt.VolumeId, "\": ", err.Error())
			}
			if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusInUse {
				break
			}
		}

		// list device after attach device
		var after []string
		files, _ = ioutil.ReadDir("/dev")
		for _, file := range files {
			if !file.IsDir() && strings.Contains(file.Name(), "vd") {
				after = append(after, file.Name())
			}
		}

		devicePaths := getDevicePath(before, after)

		if len(devicePaths) != 1 {
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			continue
		}

		devicePath = devicePaths[0]
		break
	}
	return f.Result{
		Status: "Success",
		Device: "/dev/" + devicePath,
	}
}

func (AlicloudPlugin) Detach(device string, nodeName string) f.Result {
	regionId, instanceId, err := getRegionIdAndInstanceId(nodeName)
	if err != nil {
		return f.Fail(err.Error())
	}
	raw, err := ioutil.ReadFile(credPath)
	if err != nil {
		return f.Fail("read cred file Failed: ", err.Error())
	}
	var opt f.DefaultOptions
	json.Unmarshal(raw, &opt)
	client := NewSDKClient(opt.Global.AccessKeyID, opt.Global.AccessKeySecret, instanceId)
	if client == nil {
		return f.Fail("Could not create client")
	}

	describeDisksRequest := &ecs.DescribeDisksArgs{
		RegionId: common.Region(regionId),
		DiskIds:  []string{device},
	}
	disks, _, err := client.DescribeDisks(describeDisksRequest)
	if err != nil {
		return f.Fail("Failed to list volumes: ", err.Error())
	}

	if len(disks) == 0 {
		return f.Fail("Can not find disk by Getvolumename; ", device)
	}
	disk := disks[0]
	if disk.InstanceId != "" {
		err = client.DetachDisk(disk.InstanceId, disk.DiskId)
		if err != nil {
			return f.Fail("Failed to detach: ", err.Error())
		}
	}
	return f.Succeed()
}

func getDevicePath(before, after []string) []string {
	var devicePaths []string
	for _, d := range after {
		var isNew = true
		for _, a := range before {
			if d == a {
				isNew = false
			}
		}
		if isNew {
			devicePaths = append(devicePaths, d)
		}
	}
	return devicePaths
}

func getRegionIdAndInstanceId(nodeName string) (string, string, error) {
	strs := strings.SplitN(nodeName, ".", 2)
	if len(strs) < 2 {
		return "", "", fmt.Errorf("failed to get regionID and instanceId from nodeName")
	}
	return strs[0], strs[1], nil
}

func NewSDKClient(access_key_id, access_key_secret, instanceId string) *ecs.Client {
	client := ecs.NewClient(access_key_id, access_key_secret)
	client.SetUserAgent(KUBERNETES_ALICLOUD_DISK_DRIVER + "/" + instanceId)
	return client
}

func main() {
	f.RunPlugin(&AlicloudPlugin{})
}
