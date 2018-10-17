package oss

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/denverdino/aliyungo/ecs"
	utils "github.com/AliyunContainerService/flexvolume/provider/utils"
)

type OssOptions struct {
	Bucket     string `json:"bucket"`
	Url        string `json:"url"`
	OtherOpts  string `json:"otherOpts"`
	AkId       string `json:"akId"`
	AkSecret   string `json:"akSecret"`
	VolumeName string `json:"kubernetes.io/pvOrVolumeName"`
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

// Mount Paras format:
// /usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss
// mount
// /var/lib/kubelet/pods/e000259c-4dac-11e8-a884-04163e0f011e/volumes/alicloud~oss/oss1
// {
//   "akId":"***",
//   "akSecret":"***",
//   "bucket":"oss",
//   "kubernetes.io/fsType": "",
//   "kubernetes.io/pod.name": "nginx-oss-deploy-f995c89f4-kj25b",
//   "kubernetes.io/pod.namespace":"default",
//   "kubernetes.io/pod.uid":"e000259c-4dac-11e8-a884-04163e0f011e",
//   "kubernetes.io/pvOrVolumeName":"oss1",
//   "kubernetes.io/readwrite":"rw",
//   "kubernetes.io/serviceAccount.name":"default",
//   "otherOpts":"-o max_stat_cache_size=0 -o allow_other",
//   "url":"oss-cn-hangzhou.aliyuncs.com"
// }
func (p *OssPlugin) Mount(opts interface{}, mountPath string) utils.Result {

	// logout oss paras
	opt := opts.(*OssOptions)
	argStr := ""
	for _, tmpStr := range os.Args {
		if !strings.Contains(tmpStr, "akSecret") {
			argStr += tmpStr + ", "
		}
	}
	argStr = argStr + "VolumeName: " + opt.VolumeName + ", AkId: " + opt.AkId + ", Bucket: " + opt.Bucket + ", url: " + opt.Url + ", OtherOpts: " + opt.OtherOpts
	log.Infof("Oss Plugin Mount: ", argStr)

	if !p.checkOptions(opt) {
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
		if err != nil {
			utils.FinishError("Create OSS volume fail: " + err.Error() + ", out: " + out)
		}
	}

	log.Info("Mount Oss successful: ", mountPath)
	return utils.Result{Status: "Success"}
}

// Unmount format
// /usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss
// unmount
// /var/lib/kubelet/pods/e000259c-4dac-11e8-a884-00163e0f011e/volumes/alicloud~oss/oss1
func (p *OssPlugin) Unmount(mountPoint string) utils.Result {
	log.Infof("Oss Plugin Umount: ", strings.Join(os.Args, ","))

	if !utils.IsMounted(mountPoint) {
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
		if !strings.HasPrefix(opt.OtherOpts, "-o ") {
			return false
		}
	}
	return true
}
