package utils

import "fmt"

var (
	// VERSION should be updated by hand at each release
	VERSION = "v1.9.7"

	// GITCOMMIT will be overwritten automatically by the build system
	GITCOMMIT = "HEAD"
)

func PluginVersion() string {
	return VERSION
}

func Usage() {
	fmt.Printf("In K8s Mode: " +
		"Use binary file as the first parameter, and format support:\n" +
		"    plugin init: \n" +
		"    plugin attach: for alicloud disk plugin\n" +
		"    plugin detach: for alicloud disk plugin\n" +
		"    plugin mount:  for nas, oss plugin\n" +
		"    plugin umount: for nas, oss plugin\n\n" +
		"You can refer to K8s flexvolume docs: \n")
}
