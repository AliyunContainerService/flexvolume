/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cpfs

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
        "github.com/AliyunContainerService/flexvolume/provider/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type CpfsOptions struct {
	Server     string `json:"server"`
	FileSystem string `json:"fileSystem"`
	SubPath    string `json:"subPath"`
	Options    string `json:"options"`
	VolumeName string `json:"kubernetes.io/pvOrVolumeName"`
}

const (
	CPFS_TEMP_MNTPath = "/mnt/acs_mnt/k8s_cpfs/"
)

type CpfsPlugin struct {
}

func (p *CpfsPlugin) NewOptions() interface{} {
	return &CpfsOptions{}
}

// support volume metric
func (p *CpfsPlugin) Init() utils.Result {
	driverCap := utils.DriverCapabilities{
		SupportsMetrics: true,
	}
	if utils.SupportsMetrics("cpfs") {
		return utils.InitSucceed(&driverCap)
	}
	return utils.Succeed()
}

// cpfs support mount and umount
func (p *CpfsPlugin) Mount(opts interface{}, mountPath string) utils.Result {
	log.Infof("Cpfs Volume Mount: %s", strings.Join(os.Args, ","))

	opt := opts.(*CpfsOptions)
	if err := p.checkOptions(opt); err != nil {
		log.Errorf("Cpfs, Options is illegal: %s", err.Error())
		utils.FinishError("Cpfs, Options is illegal: " + err.Error())
	}

	if utils.IsMounted(mountPath) {
		log.Infof("Cpfs, Mount Path Already Mounted, path: %s", mountPath)
		return utils.Result{Status: "Success"}
	}

	// Create Mount Path
	if err := utils.CreateDest(mountPath); err != nil {
		log.Errorf("Cpfs, Mount error with create Path fail: %s", mountPath)
		utils.FinishError("Cpfs, Mount error with create Path fail: " + mountPath)
	}

	// Do mount
	mntCmd := fmt.Sprintf("mount -t lustre %s:/%s%s %s", opt.Server, opt.FileSystem, opt.SubPath, mountPath)
	if opt.Options != "" {
		mntCmd = fmt.Sprintf("mount -t lustre -o %s %s:/%s%s %s", opt.Options, opt.Server, opt.FileSystem, opt.SubPath, mountPath)
	}
	_, err := utils.Run(mntCmd)
	if err != nil {
		if opt.SubPath != "" && opt.SubPath != "/" && strings.Contains(err.Error(), "No such file or directory") {
			p.createCpfsSubDir(opt)
			if _, err := utils.Run(mntCmd); err != nil {
				utils.FinishError("Cpfs, Mount Cpfs sub directory fail: " + err.Error())
			}
		} else {
			utils.FinishError("Cpfs, Mount cpfs fail: " + err.Error())
		}
	}

	// check mount
	if !utils.IsMounted(mountPath) {
		utils.FinishError("Check mount fail after mount:" + mountPath + ", with Command: " + mntCmd)
	}

	doCpfsConfig()
	log.Infof("CPFS Mount success on: " + mountPath + ", with Command: " + mntCmd)
	return utils.Result{Status: "Success"}
}

func doCpfsConfig() {
	configCmd := fmt.Sprintf("lctl set_param osc.*.max_rpcs_in_flight=128;lctl set_param osc.*.max_pages_per_rpc=256;lctl set_param mdc.*.max_rpcs_in_flight=256;lctl set_param mdc.*.max_mod_rpcs_in_flight=128")
	if _, err := utils.Run(configCmd); err != nil {
		log.Errorf("Cpfs, doCpfsConfig fail with error: %s", err.Error())
	}
}

// 1. mount to /mnt/acs_mnt/k8s_cpfs/temp first
// 2. run mkdir for sub directory
// 3. umount the tmep directory
func (p *CpfsPlugin) createCpfsSubDir(opt *CpfsOptions) {
	// step 1: create mount path
	rootTempPath := filepath.Join(CPFS_TEMP_MNTPath, opt.VolumeName)
	if err := utils.CreateDest(rootTempPath); err != nil {
		utils.FinishError("Create Cpfs temp Directory err: " + err.Error())
	}
	if utils.IsMounted(rootTempPath) {
		utils.Umount(rootTempPath)
	}

	// step 2: do mount
	mntCmd := fmt.Sprintf("mount -t lustre %s:/%s %s", opt.Server, opt.FileSystem, rootTempPath)
	_, err := utils.Run(mntCmd)
	if err != nil {
		utils.FinishError("CreateCpfsSubDir, Mount to temp directory fail: " + err.Error())
	}
	subPath := path.Join(rootTempPath, opt.SubPath)
	if err := utils.CreateDest(subPath); err != nil {
		utils.FinishError("CreateCpfsSubDir, Create Sub Directory err: " + err.Error())
	}

	// step 3: umount after create
	utils.Umount(rootTempPath)
	log.Infof("Create Sub Directory success for volume: %s, subpath: %s", opt.VolumeName, filepath.Join(opt.FileSystem, opt.SubPath))
}

func (p *CpfsPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Cpfs Volume Umount: %s", strings.Join(os.Args, ","))

	if !utils.IsMounted(mountPoint) {
		log.Infof("Path not mounted, skipped: %s", mountPoint)
		return utils.Succeed()
	}

	umntCmd := fmt.Sprintf("umount %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		utils.FinishError("Cpfs, Umount Cpfs Fail: " + err.Error())
	}

	log.Infof("Umount Cpfs Successful: %s, with command: %s", mountPoint, umntCmd)
	return utils.Succeed()
}

func (p *CpfsPlugin) Attach(opts interface{}, nodeName string) utils.Result {
	return utils.NotSupport()
}

func (p *CpfsPlugin) Detach(device string, nodeName string) utils.Result {
	return utils.NotSupport()
}

// Support
func (p *CpfsPlugin) Getvolumename(opts interface{}) utils.Result {
	opt := opts.(*CpfsOptions)
	return utils.Result{
		Status:     "Success",
		VolumeName: opt.VolumeName,
	}
}

// Not Support
func (p *CpfsPlugin) Waitforattach(devicePath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *CpfsPlugin) Mountdevice(mountPath string, devicePath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Cpfs options
func (p *CpfsPlugin) checkOptions(opt *CpfsOptions) error {
	// Cpfs Server url
	if opt.Server == "" {
		return errors.New("CPFS: server is empty")
	}

	// Cpfs fileSystem
	if opt.FileSystem == "" {
		return errors.New("CPFS: FileSystem is empty")
	}

	opt.SubPath = strings.TrimSpace(opt.SubPath)
	if opt.SubPath != "" && !strings.HasPrefix(opt.SubPath, "/") {
		opt.SubPath = "/" + opt.SubPath
	}
	if opt.SubPath == "" {
		opt.SubPath = "/"
	}

	opt.Options = strings.TrimSpace(opt.Options)
	return nil
}

// Not Support
func (p *CpfsPlugin) ExpandVolume(opt interface{}, devicePath, newSize, oldSize string) utils.Result {
	return utils.NotSupport()
}


// Not Support
func (p *CpfsPlugin) ExpandFS(opt interface{}, devicePath, deviceMountPath, newSize, oldSize string) utils.Result {
	return utils.NotSupport()
}
