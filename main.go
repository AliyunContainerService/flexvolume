package main

import (
	"os"
	"fmt"
	"strings"
	driver "github.com/AliyunContainerService/flexvolume/provider/driver"
	utils "github.com/AliyunContainerService/flexvolume/provider/utils"
)


// Expect to support K8s and Swarm platform
// Under K8s, plugin will run in cli mode, process running and exit after the actions.
// Under swarm, plugin will be running always, and communicate with docker by socket.
func main() {

	// get the environment of platform
	platform := os.Getenv("ACS_PLATFORM")
	if platform == "swarm" {
		driver.RunningInSwarm()
	} else {
		driver.RunK8sAction()
	}
}

// check running environment and print help
func init() {
	if len(os.Args) == 1 {
		utils.Usage()
		os.Exit(0)
	}

	argsOne := strings.ToLower(os.Args[1])
	if argsOne == "--version" || argsOne == "version" || argsOne == "-v" {
		fmt.Printf(utils.PluginVersion())
		os.Exit(0)
	}

	if argsOne == "--help" || argsOne == "help" || argsOne == "-h" {
		utils.Usage()
		os.Exit(0)
	}
}