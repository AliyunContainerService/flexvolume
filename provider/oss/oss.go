package oss

import (
	"fmt"
	"strings"
	"io/ioutil"

	"github.com/denverdino/aliyungo/ecs"
	log "github.com/Sirupsen/logrus"
	utils "gitlab.alibaba-inc.com/acs/flexvolume/provider/utils"
	"os"
)

type OssOptions struct {
	Bucket     string `json:"bucket"`
	Url        string `json:"url"`
    OtherOpts  string `json:"otherOpts"`
	AkId       string `json:"akId"`
	AkSecret   string `json:"akSecret"`
	VolumeName string `json:"volumeName"`
}

const (
	CredentialFile = "/etc/passwd-ossfs"
)

type OssPlugin struct {
	client *ecs.Client
}

func (p *OssPlugin) NewOptions() interface{} {
	return &OssOptions{}
}

func (p *OssPlugin) Init() utils.Result {
	return utils.Succeed()
}

func (p *OssPlugin) Mount(opts interface{}, mountPath string) utils.Result {

	log.Infof("Oss Plugin Mount: ", strings.Join(os.Args, ","))

	opt := opts.(*OssOptions)
	if ! p.checkOptions(opt) {
		utils.FinishError("OSS Options is illegal ")
	}

	if utils.IsMounted(mountPath) {
		return utils.Result{Status: "Success"}
	}

	// Create Mount Path
	if err := utils.CreateDest(mountPath); err != nil {
		utils.FinishError("Oss, Mount fail with create Path error: " + err.Error() + mountPath)
	}

	// Save ak file for ossfs
	if err := p.saveCredential(opt); err != nil {
		utils.FinishError("Oss, Save AK file fail: " + err.Error())
	}

	// default use allow_other
	mntCmd := fmt.Sprintf("ossfs %s %s -ourl=%s -o allow_other %s", opt.Bucket, mountPath, opt.Url, opt.OtherOpts)
	if out, err := utils.Run(mntCmd); err != nil {
		out, err = utils.Run(mntCmd + " -f")
		utils.FinishError("Create OSS volume fail: " + err.Error() + ", out: " + out)
	}

	log.Info("Mount Oss successful: ", mountPath)
	return utils.Result{Status: "Success"}
}

func (p *OssPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Oss Plugin Umount: ", strings.Join(os.Args, ","))

	if ! utils.IsMounted(mountPoint) {
		return utils.Succeed()
	}

	// do umount
	umntCmd := fmt.Sprintf("fusermount -u %s", mountPoint)
	if _, err := utils.Run(umntCmd); err != nil {
		utils.FinishError("Umount OSS Fail: " + err.Error())
	}

	log.Info("Umount Oss path successful: ", mountPoint)
	return utils.Succeed()
}

func (p *OssPlugin) Attach(opts interface{}, nodeName string) utils.Result {
	return utils.NotSupport()
}

func (p *OssPlugin) Detach(device string, nodeName string) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *OssPlugin) Getvolumename(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *OssPlugin) Waitforattach(opts interface{}) utils.Result {
	return utils.NotSupport()
}

// Not Support
func (p *OssPlugin) Mountdevice(mountPath string, opts interface{}) utils.Result {
	return utils.NotSupport()
}


// save ak file: bucket:ak_id:ak_secret
func (p *OssPlugin) saveCredential(options *OssOptions) error {

	oldContentByte := []byte{}
	if utils.IsFileExisting(CredentialFile) {
		tmpValue, err := ioutil.ReadFile(CredentialFile)
		if err != nil {
			return err
		}
		oldContentByte = tmpValue
	}

	oldContentStr := string(oldContentByte[:])
	newContentStr := ""
	for _, line := range strings.Split(oldContentStr, "\n") {
		lineList := strings.Split(line, ":")
		if len(lineList) != 3 || lineList[0] == options.Bucket {
			continue
		}
		newContentStr += line + "\n"
	}

	newContentStr = options.Bucket + ":" + options.AkId + ":" + options.AkSecret + "\n" + newContentStr
	if err := ioutil.WriteFile(CredentialFile, []byte(newContentStr), 0640); err != nil {
		log.Errorf("Save Credential File failed, %s, %s", newContentStr, err)
		return err
	}
	return nil
}

// Check oss options
func (p *OssPlugin) checkOptions(opt *OssOptions) bool {
	if opt.Url == "" || opt.Bucket == "" {
		return false
	}

	// if not input ak from user, use the default ak value
	if opt.AkId == "" || opt.AkSecret == "" {
		opt.AkId, opt.AkSecret = utils.GetLocalAK()
	}

	if opt.OtherOpts != "" {
	    if ! strings.HasPrefix(opt.OtherOpts, "-o ") {
	    	return false
		}
	}
	return true
}
