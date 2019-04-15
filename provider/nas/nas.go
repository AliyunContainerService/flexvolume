package nas

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/denverdino/aliyungo/nas"
	"github.com/AliyunContainerService/flexvolume/provider/utils"
)

type NasOptions struct {
	Server     string `json:"server"`
	Path       string `json:"path"`
	Vers       string `json:"vers"`
	Mode       string `json:"mode"`
	Opts       string `json:"options"`
	VolumeName string `json:"kubernetes.io/pvOrVolumeName"`
}

const (
	NAS_PORTNUM      = "2049"
	NAS_TEMP_MNTPath = "/mnt/acs_mnt/k8s_nas/" // used for create sub directory;
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
	if err := p.checkOptions(opt); err != nil {
		utils.FinishError("Nas, check option error: " + err.Error())
	}

	if utils.IsMounted(mountPath) {
		log.Infof("Nas, Mount Path Already Mount, options: %s", mountPath)
		return utils.Result{Status: "Success"}
	}

	// Add NAS white list if needed
	// updateNasWhiteList(opt)

	// Create Mount Path
	if err := utils.CreateDest(mountPath); err != nil {
		utils.FinishError("Nas, Mount error with create Path fail: " + mountPath)
	}

	// Do mount
	mntCmd := fmt.Sprintf("mount -t nfs -o vers=%s %s:%s %s", opt.Vers, opt.Server, opt.Path, mountPath)
	if opt.Opts != "" {
		mntCmd = fmt.Sprintf("mount -t nfs -o vers=%s,%s %s:%s %s", opt.Vers, opt.Opts, opt.Server, opt.Path, mountPath)
	}
	log.Infof("Exec Nas Mount Cdm: %s", mntCmd)
	_, err := utils.Run(mntCmd)

	// Mount to nfs Sub-directory
	if err != nil && opt.Path != "/" {
		if strings.Contains(err.Error(), "reason given by server: No such file or directory") || strings.Contains(err.Error(), "access denied by server while mounting") {
			p.createNasSubDir(opt)
			if _, err := utils.Run(mntCmd); err != nil {
				utils.FinishError("Nas, Mount Nfs sub directory fail: " + err.Error())
			}
		} else {
			utils.FinishError("Nas, Mount Nfs fail with error: " + err.Error())
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

	// do umount command
	umntCmd := fmt.Sprintf("umount %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		if strings.Contains(err.Error(), "device is busy") {
			utils.FinishError("Nas, Umount nfs Fail with device busy: " + err.Error())
		}

		// check if need force umount
		networkUnReachable := false
		noOtherPodUsed := false
		nfsServer := p.getNasServerInfo(mountPoint)
		if nfsServer != "" && ! p.isNasServerReachable(nfsServer) {
			log.Warnf("NFS, Connect to server: %s failed, umount to %s", nfsServer, mountPoint)
			networkUnReachable = true
		}
		if networkUnReachable && p.noOtherNasUser(nfsServer, mountPoint) {
			log.Warnf("NFS, Other pods is using the NAS server %s, %s", nfsServer, mountPoint)
			noOtherPodUsed = true
		}
		// force umount need both network unreachable and no other user
		if networkUnReachable && noOtherPodUsed {
			umntCmd = fmt.Sprintf("umount -f %s", mountPoint)
		}
		if _, err := utils.Run(umntCmd); err != nil {
			utils.FinishError("Nas, Umount nfs Fail: " + err.Error())
		}
	}

	log.Info("Umount nfs Successful:", mountPoint)
	return utils.Succeed()
}

func (p *NasPlugin) getNasServerInfo(mountPoint string) (string) {
	getNasServerPath := fmt.Sprintf("findmnt %s | grep %s | grep -v grep | awk '{print $2}'", mountPoint, mountPoint)
	serverAndPath, _ := utils.Run(getNasServerPath)
	serverAndPath = strings.TrimSpace(serverAndPath)

	serverInfoPartList := strings.Split(serverAndPath, ":")
	if len(serverInfoPartList) != 2 {
		log.Warnf("NFS, Get Nas Server error format: %s, %s", serverAndPath, mountPoint)
		return ""
	}
	return serverInfoPartList[0]
}

func (p *NasPlugin) noOtherNasUser(nfsServer, mountPoint string) bool {
	checkCmd := fmt.Sprintf("mount | grep -v %s | grep %s | grep -v grep | wc -l", mountPoint, nfsServer)
	if checkOut, err := utils.Run(checkCmd); err != nil {
		return false
	} else if strings.TrimSpace(checkOut) != "0" {
		return false
	}
	return true
}

func (p *NasPlugin) isNasServerReachable(url string) bool {
	conn, err := net.DialTimeout("tcp", url+":"+NAS_PORTNUM, time.Second*2)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
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

// 1. mount to /mnt/acs_mnt/k8s_nas/volumename first
// 2. run mkdir for sub directory
// 3. umount the tmep directory
func (p *NasPlugin) createNasSubDir(opt *NasOptions) {
	// step 1: create mount path
	nasTmpPath := filepath.Join(NAS_TEMP_MNTPath, opt.VolumeName)
	if err := utils.CreateDest(nasTmpPath); err != nil {
		utils.FinishError("Create Nas temp Directory err: " + err.Error())
	}
	if utils.IsMounted(nasTmpPath) {
		utils.Umount(nasTmpPath)
	}

	// step 2: do mount
	usePath := opt.Path
	mntCmd := fmt.Sprintf("mount -t nfs -o vers=%s %s:%s %s", opt.Vers, opt.Server, "/", nasTmpPath)
	_, err := utils.Run(mntCmd)
	if err != nil {
		if strings.Contains(err.Error(), "reason given by server: No such file or directory") || strings.Contains(err.Error(), "access denied by server while mounting") {
			if strings.HasPrefix(opt.Path, "/share/") {
				usePath = usePath[6:]
				mntCmd = fmt.Sprintf("mount -t nfs -o vers=%s %s:%s %s", opt.Vers, opt.Server, "/share", nasTmpPath)
				_, err := utils.Run(mntCmd)
				if err != nil {
					utils.FinishError("Nas, Mount to temp directory(with /share) fail: " + err.Error())
				}
			} else {
				utils.FinishError("Nas, maybe use fast nas, but path not startwith /share: " + err.Error())
			}
		} else {
			utils.FinishError("Nas, Mount to temp directory fail: " + err.Error())
		}
	}
	subPath := path.Join(nasTmpPath, usePath)

	if err := utils.CreateDest(subPath); err != nil {
		utils.FinishError("Nas, Create Sub Directory err: " + err.Error())
	}

	// step 3: umount after create
	utils.Umount(nasTmpPath)
	log.Info("Create Sub Directory success: ", opt.Path)
}

//
func (p *NasPlugin) checkOptions(opt *NasOptions) error {
	// NFS Server url
	if opt.Server == "" {
		return errors.New("NAS url is empty")
	}
	// check network connection
	conn, err := net.DialTimeout("tcp", opt.Server+":"+NAS_PORTNUM, time.Second*time.Duration(3))
	if err != nil {
		log.Errorf("NAS: Cannot connect to nas host: %s", opt.Server)
		return errors.New("NAS: Cannot connect to nas host: " + opt.Server)
	}
	defer conn.Close()

	// nfs server path
	if opt.Path == "" {
		opt.Path = "/"
	}
	if !strings.HasPrefix(opt.Path, "/") {
		log.Errorf("NAS: Path should be empty or start with /, %s", opt.Path)
		return errors.New("NAS: Path should be empty or start with /: " + opt.Path)
	}

	// nfs version, support 4.0, 4.1, 3.0
	// indeed, 4.1 is not available for aliyun nas now;
	if opt.Vers == "" {
		opt.Vers = "3"
	}
	if opt.Vers == "3.0" {
		opt.Vers = "3"
	}
	if opt.Vers != "4.0" && opt.Vers != "4.1" && opt.Vers != "3" {
		log.Errorf("NAS: version only support 3, 4.0 now, %s", opt.Vers)
		return errors.New("NAS: version only support 3, 4.0 now: " + opt.Vers)
	}

	// check mode
	if opt.Mode != "" {
		modeLen := len(opt.Mode)
		if modeLen != 3 {
			return errors.New("NAS: mode input format error: " + opt.Mode)
		}
		for i := 0; i < modeLen; i++ {
			if !strings.Contains(MODE_CHAR, opt.Mode[i:i+1]) {
				log.Errorf("NAS: mode is illegal, %s", opt.Mode)
				return errors.New("NAS: mode is illegal " + opt.Mode)
			}
		}
	}

	// check options
	if opt.Opts == "" {
		if opt.Vers == "3" {
			opt.Opts = "noresvport,nolock,tcp"
		} else {
			opt.Opts = "noresvport"
		}
	} else if strings.ToLower(opt.Opts) == "none" {
		opt.Opts = ""
	}

	return nil
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
