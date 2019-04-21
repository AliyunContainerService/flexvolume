package cpfs

import (
	"errors"
	"fmt"
	"github.com/AliyunContainerService/flexvolume/provider/utils"
	log "github.com/sirupsen/logrus"
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
	CPFS_TEMP_MNTPath = "/mnt/acs_mnt/k8s_cpfs/" // used for create sub directory;
)

type CpfsPlugin struct {
}

func (p *CpfsPlugin) NewOptions() interface{} {
	return &CpfsOptions{}
}

func (p *CpfsPlugin) Init() utils.Result {
	return utils.Succeed()
}

// cpfs support mount and umount
func (p *CpfsPlugin) Mount(opts interface{}, mountPath string) utils.Result {

	log.Infof("Cpfs Plugin Mount: %s", strings.Join(os.Args, ","))

	opt := opts.(*CpfsOptions)
	if err := p.checkOptions(opt); err != nil {
		log.Errorf("Cpfs, Options is illegal: %s", err.Error())
		utils.FinishError("Cpfs, Options is illegal: " + err.Error())
	}

	if utils.IsMounted(mountPath) {
		log.Infof("Cpfs, Mount Path Already Mounted, options: %s", mountPath)
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
		if opt.SubPath != "" && opt.SubPath != "/" {
			if strings.Contains(err.Error(), "No such file or directory") {
				p.createNasSubDir(opt)
				if _, err := utils.Run(mntCmd); err != nil {
					utils.FinishError("Cpfs, Mount Cpfs sub directory fail: " + err.Error())
				}
			} else {
				utils.FinishError("Nas, Mount Nfs fail with error: " + err.Error())
			}
		} else {
			utils.FinishError("Cpfs, Mount cpfs fail: " + err.Error())
		}
	}

	// check mount
	if !utils.IsMounted(mountPath) {
		utils.FinishError("Check mount fail after mount:" + mountPath)
	}
	log.Info("CPFS Mount success on: " + mountPath)
	return utils.Result{Status: "Success"}
}

// 1. mount to /mnt/acs_mnt/k8s_cpfs/temp first
// 2. run mkdir for sub directory
// 3. umount the tmep directory
func (p *CpfsPlugin) createNasSubDir(opt *CpfsOptions) {
	// step 1: create mount path
	rootTempPath := filepath.Join(CPFS_TEMP_MNTPath, opt.VolumeName)
	if err := utils.CreateDest(rootTempPath); err != nil {
		utils.FinishError("Create Nas temp Directory err: " + err.Error())
	}
	if utils.IsMounted(rootTempPath) {
		utils.Umount(rootTempPath)
	}

	// step 2: do mount
	mntCmd := fmt.Sprintf("mount -t lustre %s:/%s %s", opt.Server, opt.FileSystem, rootTempPath)
	_, err := utils.Run(mntCmd)
	if err != nil {
		utils.FinishError("Nas, Mount to temp directory fail: " + err.Error())
	}
	subPath := path.Join(rootTempPath, opt.SubPath)
	if err := utils.CreateDest(subPath); err != nil {
		utils.FinishError("Nas, Create Sub Directory err: " + err.Error())
	}

	// step 3: umount after create
	utils.Umount(rootTempPath)
	log.Info("Create Sub Directory success: ", opt.FileSystem)
}

func (p *CpfsPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Cpfs Plugin Umount: %s", strings.Join(os.Args, ","))

	if !utils.IsMounted(mountPoint) {
		log.Info("Cpfs Not mounted, not need Umount:", mountPoint)
		return utils.Succeed()
	}

	umntCmd := fmt.Sprintf("umount %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		log.Errorf("Cpfs, Umount cpfs Fail: %s, %s", err.Error(), mountPoint)
		utils.FinishError("Cpfs, Umount cpfs Fail: " + err.Error())
	}

	log.Info("Umount Cpfs Successful:", mountPoint)
	return utils.Succeed()
}

func (p *CpfsPlugin) Attach(opts interface{}, nodeName string) utils.Result {
	return utils.NotSupport()
}

func (p *CpfsPlugin) Detach(device string, nodeName string) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *CpfsPlugin) Getvolumename(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *CpfsPlugin) Waitforattach(devicePath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *CpfsPlugin) Mountdevice(mountPath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Cpfs options
func (p *CpfsPlugin) checkOptions(opt *CpfsOptions) error {
	// Cpfs Server url
	if opt.Server == "" {
		return errors.New("CPFS: server is empty")
	}

	//conn, err := net.DialTimeout("tcp", opt.Server+":"+CPFS_PORTNUM, time.Second*time.Duration(3))
	//if err != nil {
	//	log.Errorf("CPFS: Cannot connect to cpfs host: %s", opt.Server)
	//	return errors.New("CPFS: Cannot connect to cpfs host: " + opt.Server)
	//}
	//defer conn.Close()

	// Cpfs fileSystem
	if opt.FileSystem == "" {
		return errors.New("CPFS: FileSystem is empty")
	}

	opt.SubPath = strings.TrimSpace(opt.SubPath)
	if opt.SubPath != "" && !strings.HasPrefix(opt.SubPath, "/") {
		opt.SubPath = "/" + opt.SubPath
	}

	opt.Options = strings.TrimSpace(opt.Options)
	return nil
}
