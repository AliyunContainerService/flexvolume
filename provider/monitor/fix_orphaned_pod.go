package monitor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AliyunContainerService/flexvolume/provider/utils"
	log "github.com/sirupsen/logrus"
)

// issue: https://github.com/kubernetes/kubernetes/issues/60987
// global_config: true, host_config: true    --- running fix_issue_orphan_pod()
// global_config: true, host_config: false   --- not running fix_issue_orphan_pod()
// global_config: true, host_config: no_set  --- running fix_issue_orphan_pod()
// global_config: false, host_config: true   --- running fix_issue_orphan_pod()
// global_config: false, host_config: false  --- not running fix_issue_orphan_pod()
// global_config: false, host_config: no_set --- not running fix_issue_orphan_pod()
func fix_issue_orphan_pod() {
	SLEEP_SECOND := DEFAULT_SLEEP_SECOND

	for {
		if host_fix_orphaned_pod == "false" {
			time.Sleep(time.Duration(SLEEP_SECOND) * time.Second)
			continue
		}
		if host_fix_orphaned_pod == "no_set" && global_fix_orphaned_pod == false {
			time.Sleep(time.Duration(SLEEP_SECOND) * time.Second)
			continue
		}

		// got the last few lines of message file
		lines := ReadFileLines(HOST_SYS_LOG)
		flagOrphanExist := false
		podFixedList := ""
		for _, line := range lines {
			// process line which is orphan pod log.
			if strings.Contains(line, "Orphaned pod") && strings.Contains(line, "paths are still present on disk") {
				flagOrphanExist = true
				splitStr := strings.Split(line, "Orphaned pod")
				if len(splitStr) < 2 {
					log.Warnf("Orphan Pod: Error orphaned line format: %s", line)
					continue
				}
				partStr := strings.Split(splitStr[1], "\"")
				if len(partStr) < 2 {
					log.Warnf("Orphan Pod: Error line format: %s", line)
					continue
				}
				orphanUid := partStr[1]
				if len(strings.Split(orphanUid, "-")) != 5 {
					log.Warnf("Orphan Pod: Error pod Uid format: %s, %s", orphanUid, line)
					continue
				}
				if strings.Contains(podFixedList, orphanUid) {
					continue
				}
				podFixedList = podFixedList + orphanUid

				// check oss, nas, disk path;
				drivers := []string{"alicloud~disk", "alicloud~nas", "alicloud~oss", "kubernetes.io~nfs"}
				for _, driver := range drivers {
					volHostPath := "/var/lib/kubelet/pods/" + orphanUid + "/volumes/" + driver
					volPodPath := "/host/var/lib/kubelet/pods/" + orphanUid + "/volumes/" + driver
					if !utils.IsFileExisting(volPodPath) {
						continue
					}
					storagePluginDirs, err := ioutil.ReadDir(volPodPath)
					if err != nil {
						log.Errorf("Orphan Pod: read directory error: %s, %s", err.Error(), volPodPath)
						continue
					}

					for _, storagePluginDir := range storagePluginDirs {
						dirName := storagePluginDir.Name()
						mountPoint := filepath.Join(volHostPath, dirName)
						if IsHostMounted(mountPoint) {
							log.Infof("Orphan Pod: unmount directory: %s", mountPoint)
							HostUmount(mountPoint)
						}
						// remove empty directory
						if (!IsHostMounted(mountPoint)) && IsHostEmpty(mountPoint) {
							log.Infof("Orphan Pod: remove directory: %s, log info: %s", mountPoint, line)
							RemoveHostPath(mountPoint)
						} else if (!IsHostMounted(mountPoint)) && !IsHostEmpty(mountPoint) {
							log.Infof("Orphan Pod: Cannot remove directory as not empty: %s", mountPoint)
						} else {
							log.Infof("Orphan Pod: directory mounted yet: %s", mountPoint)
						}
					}
				}
				SLEEP_SECOND = DEFAULT_SLEEP_SECOND / 30
			}
		}

		// if not orphan log in message, loop slower.
		if flagOrphanExist == false {
			SLEEP_SECOND = DEFAULT_SLEEP_SECOND
		}
		time.Sleep(time.Duration(SLEEP_SECOND) * time.Second)
	}
}

// read last 2k Bytes and return lines
func ReadFileLines(fname string) []string {
	strList := []string{}

	// Open file
	file, err := os.Open(fname)
	if err != nil {
		log.Errorf("open file error: %s \n", err.Error())
		return strList
	}
	defer file.Close()

	// Get file size
	buf := make([]byte, 2000)
	stat, err := os.Stat(fname)
	if err != nil {
		log.Errorf("stat file error: %s \n", err.Error())
		return strList
	}

	start := stat.Size() - 2000
	if stat.Size() < 2000 {
		log.Infof("log file is less than 2k.")
		return strList
	}
	_, err = file.ReadAt(buf, start)
	if err != nil {
		log.Errorf("read file error: %s \n", err.Error())
		return strList
	}

	// Get first \n position
	lineIndex := 0
	for _, charValue := range buf {
		lineIndex++
		if charValue == '\n' {
			break
		}
	}
	if lineIndex >= len(buf) {
		return strList
	}

	// return the rest lines
	strLines := string(buf[lineIndex:])
	return strings.Split(strLines, "\n")
}

func IsHostMounted(mountPath string) bool {
	cmd := fmt.Sprintf("%s mount | grep \"%s type\" | grep -v grep", NSENTER_CMD, mountPath)
	out, err := utils.Run(cmd)
	if err != nil || out == "" {
		return false
	}
	return true
}

func HostUmount(mountPath string) bool {
	cmd := fmt.Sprintf("%s umount %s", NSENTER_CMD, mountPath)
	_, err := utils.Run(cmd)
	if err != nil {
		return false
	}
	return true
}

func IsHostEmpty(mountPath string) bool {
	cmd := fmt.Sprintf("%s ls %s", NSENTER_CMD, mountPath)
	out, err := utils.Run(cmd)
	if err != nil {
		return false
	}
	if out != "" {
		return false
	}
	return true
}

func RemoveHostPath(mountPath string) {
	cmd := fmt.Sprintf("%s mv %s /tmp/", NSENTER_CMD, mountPath)
	utils.Run(cmd)
}
