package monitor

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/AliyunContainerService/flexvolume/provider/utils"
)

const (
	NSENTER_CMD            = "/acs/nsenter --mount=/proc/1/ns/mnt "
	DISK_BIN               = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk"
	OSS_BIN                = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss"
	NAS_BIN                = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas"

	FLEXVOLUME_CONFIG_FILE = "/host/etc/kubernetes/flexvolume.conf"
	HOST_SYS_LOG           = "/host/var/log/messages"
	DEFAULT_SLEEP_SECOND   = 60
)

// configs for orphan pod issue
var global_fix_orphaned_pod = false
var host_fix_orphaned_pod = "no_set"

// running for plugin status check
func Monitoring() {
	// Parse fix_orphaned pod global env
	process_fix_issues := strings.ToLower(os.Getenv("FIX_ISSUES"))

	// check global config for issues;
	fix_issues_list := strings.Split(process_fix_issues, ",")
	for _, fix_issue := range fix_issues_list {
		fix_issue = strings.ToLower(fix_issue)
		if fix_issue == "fix_orphaned_pod" {
			global_fix_orphaned_pod = true
		}
	}

	// parse host flexvolume config
	go parse_flexvolume_host_config()

	// fix orphan pod with umounted path; github issue: https://github.com/kubernetes/kubernetes/issues/60987
	go fix_issue_orphan_pod()

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
func parse_flexvolume_host_config() {
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
					host_fix_orphaned_pod = "true"
					setFlag = true
					break
				} else if strings.Contains(lowLine, "fix_orphaned_pod:") && strings.Contains(lowLine, "false") {
					host_fix_orphaned_pod = "false"
					setFlag = true
					break
				}
			}
			if !setFlag {
				host_fix_orphaned_pod = "no_set"
			}
		} else {
			host_fix_orphaned_pod = "no_set"
		}

		SLEEP_SECOND := DEFAULT_SLEEP_SECOND
		time.Sleep(time.Duration(SLEEP_SECOND) * time.Second)
	}
}
