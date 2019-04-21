package monitor

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/AliyunContainerService/flexvolume/provider/utils"
	log "github.com/sirupsen/logrus"
)

// const values for monitoring
const (
	NSENTER_CMD = "/acs/nsenter --mount=/proc/1/ns/mnt "
	DISK_BIN    = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk"
	OSS_BIN     = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss"
	NAS_BIN     = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas"

	FLEXVOLUME_CONFIG_FILE = "/host/etc/kubernetes/flexvolume.conf"
	HOST_SYS_LOG           = "/host/var/log/messages"
	DEFAULT_SLEEP_SECOND   = 60
)

// configs for orphan pod issue
var globalFixOrphanedPod = false
var hostFixOrphanedPod = "no_set"

// Monitoring running for plugin status check
func Monitoring() {
	// Parse fix_orphaned pod global env
	processFixIssues := strings.ToLower(os.Getenv("FIX_ISSUES"))

	// check global config for issues;
	fixIssuesList := strings.Split(processFixIssues, ",")
	for _, fixIssue := range fixIssuesList {
		fixIssue = strings.ToLower(fixIssue)
		if fixIssue == "fix_orphaned_pod" {
			globalFixOrphanedPod = true
		}
	}

	// parse host flexvolume config
	go parseFlexvolumeHostConfig()

	// fix orphan pod with umounted path; github issue: https://github.com/kubernetes/kubernetes/issues/60987
	go fixIssueOrphanPod()

	// monitoring in loop
	for {
		version := utils.PluginVersion()
		// check Disk plugin status
		if os.Getenv("ACS_DISK") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, DISK_BIN)
			if out, err := utils.Run(mntCmd); err != nil {
				log.Printf("Warning, Monitoring disk error: %s", err.Error())
			} else if out != version {
				log.Printf("Warning, the disk plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		if os.Getenv("ACS_NAS") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, NAS_BIN)
			if out, err := utils.Run(mntCmd); err != nil {
				log.Printf("Warning, Monitoring nas error: %s", err.Error())
			} else if out != version {
				log.Printf("Warning, the nas plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		if os.Getenv("ACS_OSS") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, OSS_BIN)
			if out, err := utils.Run(mntCmd); err != nil {
				log.Printf("Warning, Monitoring oss error: %s", err.Error())
			} else if out != version {
				log.Printf("Warning, the Oss plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		time.Sleep(DEFAULT_SLEEP_SECOND * time.Second)
	}
}

// parse flexvolume global config
func parseFlexvolumeHostConfig() {
	for {
		if utils.IsFileExisting(FLEXVOLUME_CONFIG_FILE) {
			raw, err := ioutil.ReadFile(FLEXVOLUME_CONFIG_FILE)
			if err != nil {
				log.Errorf("Read flexvolume config file error: %s", err.Error())
				continue
			}
			lines := strings.Split(string(raw), "\n")
			setFlag := false
			for _, line := range lines {
				lowLine := strings.ToLower(line)

				// Parse fix_orphaned_pod config
				if strings.Contains(lowLine, "fix_orphaned_pod:") && strings.Contains(lowLine, "true") {
					hostFixOrphanedPod = "true"
					setFlag = true
					break
				} else if strings.Contains(lowLine, "fix_orphaned_pod:") && strings.Contains(lowLine, "false") {
					hostFixOrphanedPod = "false"
					setFlag = true
					break
				}
			}
			if !setFlag {
				hostFixOrphanedPod = "no_set"
			}
		} else {
			hostFixOrphanedPod = "no_set"
		}

		SLEEP_SECOND := DEFAULT_SLEEP_SECOND
		time.Sleep(time.Duration(SLEEP_SECOND) * time.Second)
	}
}
