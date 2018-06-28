package utils

import (
	"os"
	"fmt"
	"time"
)

const (
	NSENTER_CMD = "/acs/nsenter --mount=/proc/1/ns/mnt "
	DISK_BIN    = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk"
	OSS_BIN     = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~oss/oss"
	NAS_BIN     = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~nas/nas"
)

// running for plugin status check
func Monitoring() {
	for {
		version := PluginVersion()
		// check Disk plugin status
		if os.Getenv("ACS_DISK") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, DISK_BIN)
			if out, err := Run(mntCmd); err != nil {
				fmt.Printf("Warning, Monitoring disk error: %s", err.Error())
			} else if out != version {
				fmt.Printf("Warning, the disk plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		if os.Getenv("ACS_NAS") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, NAS_BIN)
			if out, err := Run(mntCmd); err != nil {
				fmt.Printf("Warning, Monitoring nas error: %s", err.Error())
			} else if out != version {
				fmt.Printf("Warning, the nas plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		if os.Getenv("ACS_OSS") == "true" {
			mntCmd := fmt.Sprintf("%s %s --version", NSENTER_CMD, OSS_BIN)
			if out, err := Run(mntCmd); err != nil {
				fmt.Printf("Warning, Monitoring oss error: %s", err.Error())
			} else if out != version {
				fmt.Printf("Warning, the Oss plugin version is not right, running: %s, expect: %s", out, version)
			}
		}

		time.Sleep(60 * time.Second)
	}
}