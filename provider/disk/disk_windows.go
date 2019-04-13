// +build windows
package disk

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/denverdino/aliyungo/common"
	"github.com/denverdino/aliyungo/ecs"
	"github.com/denverdino/aliyungo/metadata"
	"github.com/nightlyone/lockfile"
	"github.com/AliyunContainerService/flexvolume/provider/utils"
)

const (
	KUBERNETES_ALICLOUD_DISK_DRIVER = "alicloud_disk"
	VolumeDir                       = "/etc/kubernetes/volumes/disk/"
	VolumeDirRemove                 = "/etc/kubernetes/volumes/disk/remove"
	DISK_AKID                       = "/etc/.volumeak/diskAkId"
	DISK_AKSECRET                   = "/etc/.volumeak/diskAkSecret"
	DISK_ECSENPOINT                 = "/etc/.volumeak/diskEcsEndpoint"
	ECSDEFAULTENDPOINT              = "https://ecs-cn-hangzhou.aliyuncs.com"
)

type DiskOptions struct {
	VolumeName string `json:"kubernetes.io/pvOrVolumeName"`
	FsType     string `json:"kubernetes.io/fsType"`
	VolumeId   string `json:"volumeId"`
}

type DriverCapabilities struct {
	Attach         bool `json:"attach"`
	SELinuxRelabel bool `json:"selinuxRelabel"`
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
	result := utils.Succeed()
	result.Capabilities = &utils.PlugCapabilities{Attach: true}
	return result
}

func (p *DiskPlugin) Isattached(opts interface{}, nodeName string) utils.Result {
	log.Infof("Disk Plugin Isattached: %s", strings.Join(os.Args, ","))
	opt := opts.(*DiskOptions)
	cmd := fmt.Sprintf("(get-volume | where-object FilesystemLabel -eq \"%s\").DriveLetter", opt.VolumeName)
	if out, err := utils.Run(cmd); err == nil {
		result := utils.Result{Status: "Success"}
		result.Attached = len(out) > 0
		return result
	}
	utils.FinishError("command failed")
	return utils.Result{}
}

func (p *DiskPlugin) Getvolumename(opts interface{}) utils.Result {
	log.Infof("Disk Plugin Getvolumename: %s", strings.Join(os.Args, ","))
	return utils.NotSupport()
}

// attach with NodeName and Options
// nodeName: regionId.instanceId, exammple: cn-hangzhou.i-bp12gei4ljuzilgwzahc
// options: {"kubernetes.io/fsType": "", "kubernetes.io/pvOrVolumeName": "", "kubernetes.io/readwrite": "", "volumeId":""}
func (p *DiskPlugin) Attach(opts interface{}, nodeName string) utils.Result {
	log.Infof("Disk Plugin Attach: %s", strings.Join(os.Args, ","))

	// Step 0: Check disk is attached on this host
	// resolve kubelet restart issue
	opt := opts.(*DiskOptions)
	cmd := fmt.Sprintf("(get-disk -number (get-partition -driveletter (get-volume -FilesystemLabel \"%s\").DriveLetter).disknumber).path", opt.VolumeName)
	if out, err := utils.Run(cmd); err == nil && len(out) > 0 {
		devicePath := out[0]
		log.Infof("Disk Already Attached, DiskId: %s, Device: %s", opt.VolumeName, devicePath)
		return utils.Result{Status: "Success", Device: devicePath}
	}

	// Step 1: init ecs client and parameters
	p.initEcsClient()
	regionId, instanceId, err := utils.GetRegionAndInstanceId()
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
		log.Info("detach successfully")
	}

	// Step 3: wait for Detach
	for i := 0; i < 15; i++ {
		disks, _, err := p.client.DescribeDisks(describeDisksRequest)
		if err != nil {
			utils.FinishError("Could not get Disk again " + opt.VolumeId + ", with error: " + err.Error())
		}
		if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusAvailable {
			break
		}
		if i == 14 {
			utils.FinishError("Detach disk timeout, failed: " + opt.VolumeId)
		}
		time.Sleep(2000 * time.Millisecond)
	}
	log.Infof("Disk is ready to attach: ", opt.VolumeName, opt.VolumeId, opt.FsType)

	// multi disk attach at the same time
	// lck file created under /tmp/
	lockfileName := "lockfile-disk.lck"
	lock, err := lockfile.New(filepath.Join(os.TempDir(), lockfileName))
	if err != nil {
		utils.FinishError("Lockfile New failed, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
	}
	err = lock.TryLock()
	if err != nil {
		utils.FinishError("Lockfile failed, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
	}
	defer lock.Unlock()

	// Step 4: Attach Disk, list device before attach disk
	before := GetCurrentDevices()
	log.Info("trying to attach")
	if err = p.client.AttachDisk(attachRequest); err != nil {
		utils.FinishError("Attach failed, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
	}

	// Step 5: wait for attach
	for i := 0; i < 15; i++ {
		disks, _, err := p.client.DescribeDisks(describeDisksRequest)
		if err != nil {
			utils.FinishError("Attach describe error, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", err: " + err.Error())
		}
		if len(disks) >= 1 && disks[0].Status == ecs.DiskStatusInUse {
			break
		}
		if i == 14 {
			utils.FinishError("Attach timeout, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName)
		}
		time.Sleep(2000 * time.Millisecond)
	}

	// Step 6: Analysis attach device, list device after attach device
	for i := 0; i < 15; i++ {
		after := GetCurrentDevices()
		devicePaths := getDevicePath(before, after)
		if i == 9 {
			utils.FinishError("Attach Success, but get DevicePath error1, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", DevicePaths: " + strings.Join(devicePaths, ",") + ", After: " + strings.Join(after, ","))
		}
		if len(devicePaths) == 2 && strings.HasPrefix(devicePaths[1], devicePaths[0]) {
			devicePath = devicePaths[1]
			break
		} else if len(devicePaths) == 1 {
			devicePath = devicePaths[0]
			break
		} else if len(devicePaths) == 0 {
			time.Sleep(2 * time.Second)
		} else {
			utils.FinishError("Attach Success, but get DevicePath error2, DiskId: " + opt.VolumeId + ", Volume: " + opt.VolumeName + ", DevicePaths: " + strings.Join(devicePaths, ",") + ", After: " + strings.Join(after, ","))
		}
	}

	// save volume info to file
	if err := saveVolumeConfig(opt); err != nil {
		log.Errorf("Save volume config failed: " + err.Error())
	}

	log.Infof("Attach successful, DiskId: %s, Volume: %s, Device: %s", opt.VolumeId, opt.VolumeName, devicePath)

	log.Infof("Taking deivce %s online", devicePath)
	_, err = utils.Run(fmt.Sprintf("set-disk -path \"%s\" -isoffline 0", devicePath))
	if err != nil {
		utils.FinishError("Taking deivce online failed: " + err.Error())
	}
	_, err = utils.Run(fmt.Sprintf("set-disk -path \"%s\" -isreadonly 0", devicePath))
	if err != nil {
		utils.FinishError("Setting deivce rw failed: " + err.Error())
	}

	log.Infof("Formatting disk")
	_, err = utils.Run(fmt.Sprintf("get-disk -path \"%s\" | initialize-disk -passthru | new-partition -assigndriveletter -usemaximumsize | format-volume -filesystem ntfs -newfilesystemlabel \"%s\"", devicePath, opt.VolumeName))
	if err != nil {
		utils.FinishError("Format disk failed: " + err.Error())
	}
	return utils.Result{
		Status: "Success",
		Device: devicePath,
	}
}

// Get devices like /dev/vd**
func GetCurrentDevices() []string {
	devices, _ := utils.Run("(get-disk | where Model -eq \"VirtIO\" | select path).path")
	return devices
}

// current kubelet call detach not provide plugin spec;
// this issue is tracked by: https://github.com/kubernetes/kubernetes/issues/52590
func (p *DiskPlugin) Detach(volumeName string, nodeName string) utils.Result {
	log.Infof("Disk Plugin Detach: %s", strings.Join(os.Args, ","))

	log.Infof("Taking device offline")
	//for unknown reason, ecs detachDisk faild sometimes, and detach'll be call again.
	//we should skip offline process otherwise detach never succeed.
	_, err := utils.Run(fmt.Sprintf("get-volume -filesystemlabel \"%s\"", volumeName))
	if err == nil {
		_, err = utils.Run(fmt.Sprintf("set-disk -number (get-partition -driveletter (get-volume -filesystemlabel \"%s\").driveletter).disknumber -isoffline 1", volumeName))
		if err != nil {
			utils.FinishError("Taking deivce offline failed: " + err.Error())
		}
	} else {
		log.Infof("volume %s not found, probably disk already offline", volumeName)
	}
	// Step 1: init ecs client
	p.initEcsClient()
	regionId, instanceId, err := utils.GetRegionAndInstanceId()
	if err != nil {
		utils.FinishError("Detach with get regionid/instanceid error: " + err.Error())
	}

	// step 2: get diskid
	diskId := volumeName
	tmpDiskId := getVolumeConfig(volumeName)
	if tmpDiskId != "" && tmpDiskId != volumeName {
		diskId = tmpDiskId
	}

	// Step 3: check disk
	p.client.SetUserAgent(KUBERNETES_ALICLOUD_DISK_DRIVER + "/" + instanceId)
	describeDisksRequest := &ecs.DescribeDisksArgs{
		RegionId: common.Region(regionId),
		DiskIds:  []string{diskId},
	}
	disks, _, err := p.client.DescribeDisks(describeDisksRequest)
	if err != nil {
		utils.FinishError("Failed to list Volume: " + volumeName + ", DiskId: " + diskId + ", with error: " + err.Error())
	}
	if len(disks) == 0 {
		log.Info("No Need Detach, Volume: ", volumeName, ", DiskId: ", diskId, " is not exist")
		return utils.Succeed()
	}

	// Step 4: Detach disk
	disk := disks[0]
	if disk.InstanceId != "" {
		// only detach disk on self instance
		if disk.InstanceId != instanceId {
			log.Info("Skip Detach, Volume: ", volumeName, ", DiskId: ", diskId, " is attached on: ", disk.InstanceId)
			return utils.Succeed()
		}

		err = p.client.DetachDisk(disk.InstanceId, disk.DiskId)
		if err != nil {
			utils.FinishError("Disk, Failed to detach: " + err.Error())
		}
	}

	// step 5: remove volume config file
	removeVolumeConfig(volumeName)

	log.Info("Detach Successful, Volume: ", volumeName, ", DiskId: ", diskId, ", NodeName: ", nodeName)
	return utils.Succeed()
}

// Not Support
func (p *DiskPlugin) Mount(opts interface{}, mountPath string) utils.Result {
	log.Infof("Disk Plugin Mount: %s", strings.Join(os.Args, ","))
	opt := opts.(*DiskOptions)
	out, err := utils.Run(fmt.Sprintf("(get-volume -FilesystemLabel \"%s\").driveletter", opt.VolumeName))
	if err != nil {
		utils.FinishError("get mount device failed: " + err.Error())
	}
	if len(out) <= 0 {
		utils.FinishError("mount device not found")
	}
	driveletter := out[0]
	/*
		err = os.Symlink(driveletter+":", mountPath)
		if err != nil {
			utils.FinishError("create symbolic link failed: " + err.Error())
		}*/

	utils.Run(fmt.Sprintf("(get-item %s).Delete()", mountPath))
	cmd := exec.Command("cmd", "/C", fmt.Sprintf("mklink /D %s %s", mountPath, driveletter+":"))
	if err := cmd.Run(); err != nil {
		utils.FinishError("create symbolic link failed: " + err.Error())
	}
	return utils.Succeed()
}

// Not support
func (p *DiskPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Disk Plugin Unmount: %s", strings.Join(os.Args, ","))
	_, err := utils.Run(fmt.Sprintf("(get-item %s).Delete()", mountPoint))
	if err != nil {
		utils.FinishError("delete symbolic link failed: " + err.Error())
	}
	return utils.Succeed()
}

// Not Support
func (p *DiskPlugin) Waitforattach(opts interface{}) utils.Result {
	log.Infof("Disk Plugin Waitforattach: %s", strings.Join(os.Args, ","))
	return utils.NotSupport()
}

func (p *DiskPlugin) Mountdevice(mountPath string, mountDevice string, opts interface{}) utils.Result {
	log.Infof("Disk Plugin Mountdevice: %s", strings.Join(os.Args, ","))
	opt := opts.(*DiskOptions)

	out, err := utils.Run(fmt.Sprintf("(get-disk -path \"%s\").NumberOfPartitions", mountDevice))
	if err != nil {
		utils.FinishError("Get disk information failed: " + err.Error())
	}
	if len(out) <= 0 {
		utils.FinishError("Get disk information failed: disk not found")
	}
	numOfPartition, _ := strconv.Atoi(out[0])
	if numOfPartition > 0 {
		log.Infof("Disk already fomatted")
		return utils.Succeed()
	}
	log.Infof("Formatting disk")
	_, err = utils.Run(fmt.Sprintf("get-disk -path \"%s\" | initialize-disk -passthru | new-partition -assigndriveletter -usemaximumsize | format-volume -filesystem ntfs -newfilesystemlabel \"%s\"", mountDevice, opt.VolumeName))
	if err != nil {
		utils.FinishError("Format disk failed: " + err.Error())
	}
	return utils.Succeed()
}

func (p *DiskPlugin) Unmountdevice(mountDevice string) utils.Result {
	log.Infof("Disk Plugin Unmountdevice: %s", strings.Join(os.Args, ","))
	//do nothing
	return utils.Succeed()
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
func (p *DiskPlugin) GetDiskLocalConfig() (string, string, string) {
	accessKeyID, accessSecret, ecsEndpoint := "", "", ""

	if utils.IsFileExisting(DISK_AKID) && utils.IsFileExisting(DISK_AKSECRET) && utils.IsFileExisting(DISK_ECSENPOINT) {
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

// endpoint: env variable first; /etc/.volumeak/diskEcsEndpoint second, overseas region third;
func newEcsClient(access_key_id, access_key_secret, access_token, ecs_endpoint string) *ecs.Client {
	m := metadata.NewMetaData(nil)
	region, err := m.Region()
	if err != nil {
		region = string(DEFAULT_REGION)
	}

	// use environment endpoint first;
	if ep := os.Getenv("ECS_ENDPOINT"); ep != "" {
		ecs_endpoint = ep
	}

	client := ecs.NewECSClientWithEndpointAndSecurityToken(ecs_endpoint, access_key_id, access_key_secret, access_token, common.Region(region))
	client.SetUserAgent(KUBERNETES_ALICLOUD_IDENTITY)

	return client
}

// get diskID
func getVolumeConfig(volumeName string) string {
	volumeFile := path.Join(VolumeDir, volumeName+".conf")
	if !utils.IsFileExisting(volumeFile) {
		return ""
	}

	value, err := ioutil.ReadFile(volumeFile)
	if err != nil {
		return ""
	}

	volumeId := strings.TrimSpace(string(value))
	return volumeId
}

// save diskID and volume name
func saveVolumeConfig(opt *DiskOptions) error {
	if err := utils.CreateDest(VolumeDir); err != nil {
		return err
	}
	if err := utils.CreateDest(VolumeDirRemove); err != nil {
		return err
	}
	if err := removeVolumeConfig(opt.VolumeName); err != nil {
		return err
	}

	volumeFile := path.Join(VolumeDir, opt.VolumeName+".conf")
	if err := ioutil.WriteFile(volumeFile, []byte(opt.VolumeId), 0644); err != nil {
		return err
	}
	return nil
}

// move config file to remove dir
func removeVolumeConfig(volumeName string) error {
	volumeFile := path.Join(VolumeDir, volumeName+".conf")
	if utils.IsFileExisting(volumeFile) {
		timeStr := time.Now().Format("2006-01-02-15-04-05")
		removeFile := path.Join(VolumeDirRemove, volumeName+"-"+timeStr+".conf")
		if err := os.Rename(volumeFile, removeFile); err != nil {
			return err
		}
	}
	return nil
}
