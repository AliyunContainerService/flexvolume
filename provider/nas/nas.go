package nas

import (
	"fmt"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	utils "github.com/AliyunContainerService/flexvolume/provider/utils"
	"github.com/denverdino/aliyungo/nas"
	log "github.com/sirupsen/logrus"
	"os"
)

type NasOptions struct {
	Server     string `json:"server"`
	Path       string `json:"path"`
	Vers       string `json:"vers"`
	Mode       string `json:"mode"`
	VolumeName string `json:"volumeName"`
}

const (
	NAS_PORTNUM      = "2049"
	NAS_TEMP_MNTPath = "/mnt/acs_mnt/k8s_nas/temp" // used for create sub directory;
	MODE_CHAR        = "01234567"
)

type NasPlugin struct {
	client *nas.Client
}

func (p *NasPlugin) NewOptions() interface{} {
	return &NasOptions{}
}

func (p *NasPlugin) Init() utils.Result {
	return utils.Succeed()
}

// nas support mount and umount
func (p *NasPlugin) Mount(opts interface{}, mountPath string) utils.Result {

	log.Infof("Nas Plugin Mount: %s", strings.Join(os.Args, ","))

	opt := opts.(*NasOptions)
	if !p.checkOptions(opt) {
		utils.FinishError("Nas, Options is illegal")
	}

	if utils.IsMounted(mountPath) {
		log.Infof("Nas, Mount Path Already Mount, options: %s", mountPath)
		return utils.Result{Status: "Success"}
	}

	// TODO: Add NAS white list if needed
	// updateNasWhiteList(opt)

	// Create Mount Path
	if err := utils.CreateDest(mountPath); err != nil {
		utils.FinishError("Nas, Mount error with create Path fail: " + mountPath)
	}

	// Do mount
	mntCmd := fmt.Sprintf("mount -t nfs -o vers=%s %s:%s %s", opt.Vers, opt.Server, opt.Path, mountPath)
	_, err := utils.Run(mntCmd)

	// Mount to nfs Sub-directory
	if err != nil && strings.Contains(err.Error(), "reason given by server: No such file or directory") && opt.Path != "/" {
		p.createNasSubDir(opt)
		if _, err := utils.Run(mntCmd); err != nil {
			utils.FinishError("Nas, Mount Nfs sub directory fail: " + err.Error())
		}
		// mount error
	} else if err != nil {
		utils.FinishError("Nas, Mount nfs fail: " + err.Error())
	}

	// change the mode
	if opt.Mode != "" && opt.Path != "/" {
		var wg1 sync.WaitGroup
		wg1.Add(1)

		go func(*sync.WaitGroup) {
			cmd := fmt.Sprintf("chmod -R %s %s", opt.Mode, mountPath)
			if _, err := utils.Run(cmd); err != nil {
				log.Errorf("Nas chmod cmd fail: %s %s", cmd, err)
			} else {
				log.Infof("Nas chmod cmd success: %s", cmd)
			}
			wg1.Done()
		}(&wg1)

		if waitTimeout(&wg1, 1) {
			log.Infof("Chmod use more than 1s, running in Concurrency: %s", mountPath)
		}
	}

	// chmod for sub directory
	//if opt.Mode != "" && opt.Path != "/" {
	//	mntCmd := fmt.Sprintf("chmod -R %s %s", opt.Mode, mountPath)
	//	if _, err := utils.Run(mntCmd); err != nil {
	//		utils.FinishError("Nas, chmod sub-directory fail: " + err.Error())
	//	}
	//}

	// check mount
	if !utils.IsMounted(mountPath) {
		utils.FinishError("Check mount fail after mount:" + mountPath)
	}
	log.Info("Mount success on: " + mountPath)
	return utils.Result{Status: "Success"}
}

func (p *NasPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Nas Plugin Umount: %s", strings.Join(os.Args, ","))

	if !utils.IsMounted(mountPoint) {
		return utils.Succeed()
	}

	umntCmd := fmt.Sprintf("umount -f %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		utils.FinishError("Nas, Umount nfs Fail: " + err.Error())
	}

	log.Info("Umount nfs Successful: %s", mountPoint)
	return utils.Succeed()
}

func (p *NasPlugin) Attach(opts interface{}, nodeName string) utils.Result {
	return utils.NotSupport()
}

func (p *NasPlugin) Detach(device string, nodeName string) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *NasPlugin) Getvolumename(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *NasPlugin) Waitforattach(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *NasPlugin) Mountdevice(mountPath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}

// 1. mount to /mnt/acs_mnt/k8s_nas/tmp first
// 2. run mkdir for sub directory
// 3. umount the tmep directory
func (p *NasPlugin) createNasSubDir(opt *NasOptions) {
	// create mount path
	if err := utils.CreateDest(NAS_TEMP_MNTPath); err != nil {
		utils.FinishError("Create Nas temp Directory err: " + err.Error())
	}
	if utils.IsMounted(NAS_TEMP_MNTPath) {
		utils.Umount(NAS_TEMP_MNTPath)
	}

	// do mount
	mntCmd := fmt.Sprintf("mount -t nfs -o vers=%s %s:%s %s", opt.Vers, opt.Server, "/", NAS_TEMP_MNTPath)
	_, err := utils.Run(mntCmd)
	if err != nil {
		utils.FinishError("Nas, Mount to temp directory fail: " + err.Error())
	}
	subPath := path.Join(NAS_TEMP_MNTPath, opt.Path)
	if err := utils.CreateDest(subPath); err != nil {
		utils.FinishError("Nas, Create Sub Directory err: " + err.Error())
	}

	// umount after create
	utils.Umount(NAS_TEMP_MNTPath)
	log.Info("Create Sub Directory success: ", opt.Path)
}

//
func (p *NasPlugin) checkOptions(opt *NasOptions) bool {
	// NFS Server url
	if opt.Server == "" {
		return false
	}
	// check network connection
	conn, err := net.DialTimeout("tcp", opt.Server+":"+NAS_PORTNUM, time.Second*time.Duration(3))
	if err != nil {
		log.Errorf("NAS: Cannot connect to nas host: %s", opt.Server)
		return false
	}
	defer conn.Close()

	// nfs server path
	if opt.Path == "" {
		opt.Path = "/"
	}
	if !strings.HasPrefix(opt.Path, "/") {
		log.Errorf("NAS: Path should be empty or start with /, %s", opt.Path)
		return false
	}

	// nfs version, support 4.0, 4.1, 3.0
	if opt.Vers != "4.0" && opt.Vers != "4.1" && opt.Vers != "3.0" {
		return false
	}

	// check mode
	if opt.Mode != "" {
		modeLen := len(opt.Mode)
		if modeLen != 3 {
			return false
		}
		for i := 0; i < modeLen; i++ {
			if !strings.Contains(MODE_CHAR, opt.Mode[i:i+1]) {
				return false
			}
		}
	}
	return true
}

func waitTimeout(wg *sync.WaitGroup, timeout int) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false
	case <-time.After(time.Duration(timeout) * time.Second):
		return true
	}

}
