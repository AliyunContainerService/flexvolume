package disk

import (
	"os"
	"fmt"
	"path"
	"time"
	"errors"
	"strings"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/denverdino/aliyungo/common"
	utils "github.com/AliyunContainerService/flexvolume/provider/utils"
)


const (
	KUBERNETES_ALICLOUD_DISK_DRIVER = "alicloud_disk"
	VolumeDir                       = "/etc/kubernetes/volumes/disk/"
	VolumeDirRemove                 = "/etc/kubernetes/volumes/disk/remove"
	DISK_AKID                       = "/etc/.volumeak/diskAkId"
    DISK_AKSECRET                   = "/etc/.volumeak/diskAkSecret"
    DISK_ECSENPOINT                 = "/etc/.volumeak/diskEcsEndpoint"
)

type DiskOptions struct {
	VolumeName  string `json:"kubernetes.io/pvOrVolumeName"`
	FsType      string `json:"kubernetes.io/fsType"`
	VolumeId    string `json:"volumeId"`
}

var KUBERNETES_ALICLOUD_IDENTITY = fmt.Sprintf("Kubernetes.Alicloud/Flexvolume.Disk-%s", utils.PluginVersion())
var DEFAULT_REGION = common.Hangzhou

type DiskPlugin struct {
	client *ecs.Client
}

func (p *DiskPlugin) NewOptions() interface{} {
	return &DiskOptions{}
}

func (p *DiskPlugin) Init() utils.Result {
	return utils.Succeed()
}

// attach with NodeName and Options
// nodeName: regionId.instanceId, exammple: cn-hangzhou.i-bp12gei4ljuzilgwzahc
// options: {"kubernetes.io/fsType": "", "kubernetes.io/pvOrVolumeName": "", "kubernetes.io/readwrite": "", "volumeId":""}
func (p *DiskPlugin) Attach(opts interface{}, nodeName string) utils.Result {

	log.Infof("Disk Plugin Attach: %s", strings.Join(os.Args, ","))

	// Step 0: Check disk is attached on this host
	// resolve kubelet restart issue
	opt := opts.(*DiskOptions)
	cmd := fmt.Sprintf("mount | grep alicloud~disk/%s", opt.VolumeName)
	if out, err := utils.Run(cmd); err == nil {
		devicePath := strings.Split(strings.TrimSpace(out), " ")[0]
		log.Infof("Disk Already Attached, DiskId: %s, Device: %s", opt.VolumeName, devicePath)
		return utils.Result{ Status: "Success", Device: devicePath }
	}

	// Step 1: init ecs client and parameters
	p.initEcsClient()
	regionId, instanceId, err := utils.GetRegionIdAndInstanceId(nodeName)
	if err != nil {
		utils.FinishError("Disk, Parse node region/name error: " + nodeName + err.Error())
	}
	p.client.SetUserAgent(KUBERNETES_ALICLOUD_DISK_DRIVER + "/" + instanceId)
	attachRequest := &ecs.AttachDiskArgs{
		InstanceId: instanceId,
		DiskId:     opt.VolumeId,
	}

	// Step 2: Detach disk first
	var devicePath string
	describeDisksRequest := &ecs.DescribeDisksArgs{
		DiskIds:  []string{opt.VolumeId},
		RegionId: common.Region(regionId),
	}
	// call detach to ensure work after node reboot
	disks, _, err := p.client.DescribeDisks(describeDisksRequest)
	if err != nil {
		utils.FinishError("Disk, Can not get disk: " + opt.VolumeId + ", with error:" + err.Error())
	}
	if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusInUse {
		err = p.client.DetachDisk(disks[0].InstanceId, disks[0].DiskId)
		if err != nil {
			utils.FinishError("Disk, Failed to detach: " + err.Error())
		}
	}

	// Step 3: wait for Detach
	var lastErr error
	var retryDetachCount = 3
	for {
		retryDetachCount--
		if retryDetachCount < 0 {
			utils.FinishError("Detach disk timeout, failed: " + lastErr.Error())
		}
		time.Sleep(1000 * time.Millisecond)
		disks, _, err := p.client.DescribeDisks(describeDisksRequest)
		if err != nil {
			utils.FinishError("Could not get Disk again " +  opt.VolumeId + ", with error: " + err.Error())
		}
		if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusAvailable {
			break
		}
		lastErr = errors.New(fmt.Sprintf("%+v\n",disks))
	}
	log.Infof("Disk is ready to attach: %s", opt.VolumeName, opt.VolumeId, opt.FsType)


	// Step 4: Attach Disk, list device before attach disk
	var before []string
	files, _ := ioutil.ReadDir("/dev")
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), "vd") {
			before = append(before, file.Name())
		}
	}
	if err = p.client.AttachDisk(attachRequest); err != nil {
		utils.FinishError("Attach failed, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
	}

	// Step 5: wait for attach
	var retryAttachCount = 3
	for {
		retryAttachCount--
		if retryAttachCount < 0 {
			utils.FinishError("Attach timeout, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName)
		}
		time.Sleep(1000 * time.Millisecond)
		disks, _, err := p.client.DescribeDisks(describeDisksRequest)
		if err != nil {
			utils.FinishError("Attach describe error, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
		}
		if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusInUse {
			break
		}
		lastErr = errors.New(fmt.Sprintf("%+v\n",disks))
	}

	// Step 6: Analysis attach device, list device after attach device
	var after []string
	files, _ = ioutil.ReadDir("/dev")
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), "vd") {
			after = append(after, file.Name())
		}
	}
	devicePaths := getDevicePath(before, after)
	if len(devicePaths) == 2 && strings.HasPrefix(devicePaths[1],devicePaths[0]) {
		devicePath = devicePaths[1]
	}else if len(devicePaths) == 1 {
		devicePath = devicePaths[0]
	}else{
		utils.FinishError("Attach Success, but get DevicePath error, DiskId: "  + opt.VolumeId + ", Volume: " + opt.VolumeName)
	}

	// save volume config to file
	//if err := saveVolumeConfig(opt); err != nil {
	//	utils.FinishError("Save volume config failed: " + err.Error())
	//}

	log.Infof("Attach successful, DiskId: %s, Volume: %s, Device: %s", opt.VolumeId, opt.VolumeName, devicePath)
	return utils.Result{
		Status: "Success",
		Device: "/dev/" + devicePath,
	}
}

// current kubelet call detach not provide plugin spec;
// this issue is tracked by: https://github.com/kubernetes/kubernetes/issues/52590
func (p *DiskPlugin) Detach(volumeName string, nodeName string) utils.Result {
	log.Infof("Disk Plugin Detach: %s", strings.Join(os.Args, ","))

	// Step 1: init ecs client
	p.initEcsClient()
	regionId, instanceId, err := utils.GetRegionIdAndInstanceId(nodeName)
	if err != nil {
		utils.FinishError("Detach with describe error: " + err.Error())
	}

	// Step 2: check disk
	p.client.SetUserAgent(KUBERNETES_ALICLOUD_DISK_DRIVER + "/" + instanceId)
	describeDisksRequest := &ecs.DescribeDisksArgs{
		RegionId: common.Region(regionId),
		DiskIds:  []string{volumeName},
	}
	disks, _, err := p.client.DescribeDisks(describeDisksRequest)
	if err != nil || len(disks) == 0 {
		utils.FinishError("Failed to list Volume: " + volumeName + ", with error: " + err.Error())
	}

	// Step 3: Detach disk
	disk := disks[0]
	if disk.InstanceId != "" {
		// only detach disk on self instance
		if disk.InstanceId != instanceId {
			log.Info("Skip Detach, Volume: %s", volumeName, " is attached on: %s", disk.InstanceId)
			return utils.Succeed()
		}

		err = p.client.DetachDisk(disk.InstanceId, disk.DiskId)
		if err != nil {
			utils.FinishError("Disk, Failed to detach: " + err.Error())
		}
	}

	log.Info("Detach Successful, Volume: %s", volumeName, ", NodeName: ", nodeName)
	return utils.Succeed()
}


// Not Support
func (p *DiskPlugin) Mount(opts interface{}, mountPath string) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *DiskPlugin) Unmount(mountPoint string) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *DiskPlugin) Getvolumename(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *DiskPlugin) Waitforattach(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *DiskPlugin) Mountdevice(mountPath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

//
func (p *DiskPlugin) initEcsClient() {
	accessKeyID, accessSecret, accessToken, ecsEndpoint := "", "", "", ""
	// Apsara Stack use local config file
	accessKeyID, accessSecret, ecsEndpoint = p.GetDiskLocalConfig()

	// the common environment
	if accessKeyID == "" || accessSecret == "" {
		accessKeyID, accessSecret, accessToken = utils.GetDefaultAK()
	}

	p.client = newEcsClient(accessKeyID, accessSecret, accessToken, ecsEndpoint)
	if p.client == nil {
		utils.FinishError("New Ecs Client error, ak_id: " + accessKeyID)
	}
}

// read disk config from local file
func (p *DiskPlugin)GetDiskLocalConfig() (string, string, string) {
	accessKeyID, accessSecret, ecsEndpoint := "", "", ""

	if utils.IsFileExisting(DISK_AKID) && utils.IsFileExisting(DISK_AKSECRET) && utils.IsFileExisting(DISK_ECSENPOINT){
		raw, err := ioutil.ReadFile(DISK_AKID)
		if err != nil {
			log.Error("Read disk AK ID file error:", err.Error())
			return "", "", ""
		}
		accessKeyID = string(raw)

		raw, err = ioutil.ReadFile(DISK_AKSECRET)
		if err != nil {
			log.Error("Read disk AK Secret file error:", err.Error())
			return "", "", ""
		}
		accessSecret = string(raw)

		raw, err = ioutil.ReadFile(DISK_ECSENPOINT)
		if err != nil {
			log.Error("Read disk ecs Endpoint file error:", err.Error())
			return "", "", ""
		}
		ecsEndpoint = string(raw)
	}
	return strings.TrimSpace(accessKeyID), strings.TrimSpace(accessSecret), strings.TrimSpace(ecsEndpoint)
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

func newEcsClient(access_key_id, access_key_secret, access_token, ecs_endpoint string) *ecs.Client {
	client := ecs.NewClient(access_key_id, access_key_secret)
	if access_token != "" {
		client.SetSecurityToken(access_token)
		client.SetRegionID(DEFAULT_REGION)
    }
    if ecs_endpoint != "" {
    	client.SetEndpoint(ecs_endpoint)
	}

    client.SetUserAgent(KUBERNETES_ALICLOUD_IDENTITY)
	return client
}

// save diskID and volume name
func saveVolumeConfig (opt *DiskOptions) error {
	if err := utils.CreateDest(VolumeDir); err != nil {
		return err
	}
	if err := utils.CreateDest(VolumeDirRemove); err != nil {
		return err
	}
	if err := removeVolumeConfig(opt.VolumeName); err != nil {
		return err
	}

	volumeFile := path.Join(VolumeDir, opt.VolumeName + ".json")
	if err := utils.WriteJosnFile(*opt, volumeFile); err != nil {
		return err
	}
	return nil
}

// move config file to remove dir
func removeVolumeConfig(volumeName string) error {
	volumeFile := path.Join(VolumeDir, volumeName + ".json")
	if utils.IsFileExisting(volumeFile) {
		timeStr := time.Now().Format("-2006-01-02-15:04:05")
		removeFile := path.Join(VolumeDirRemove, volumeName + "-" + timeStr + ".json")
		if err := os.Rename(volumeFile, removeFile); err != nil {
			return err
		}
	}
	return nil
}
