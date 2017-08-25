package common

import (
	"encoding/json"
	"fmt"
	"os"
)

type Result struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Device  string `json:"device,omitempty"`
}

type DefaultOptions struct {
	Global struct {
		KubernetesClusterTag string

		AccessKeyID     string `json:"accessKeyID"`
		AccessKeySecret string `json:"accessKeySecret"`
		Region          string `json:"region"`
	}
}

type FlexVolumePlugin interface {
	NewOptions() interface{}
	Init() Result
	Attach(opt interface{}, nodeName string) Result
	Detach(device string, nodeName string) Result
}

func Succeed(a ...interface{}) Result {
	return Result{
		Status:  "Success",
		Message: fmt.Sprint(a...),
	}
}

func NotSupport(a ...interface{}) Result {
	return Result{
		Status:  "Not supported",
		Message: fmt.Sprint(a...),
	}
}

func Fail(a ...interface{}) Result {
	return Result{
		Status:  "Failure",
		Message: fmt.Sprint(a...),
	}
}

func finish(result Result) {
	code := 1
	if result.Status == "Success" {
		code = 0
	}
	res, err := json.Marshal(result)
	if err != nil {
		fmt.Println("{\"status\":\"Failure\",\"message\":", err.Error(), "}")
	} else {
		fmt.Println(string(res))
	}
	os.Exit(code)
}

func RunPlugin(plugin FlexVolumePlugin) {
	if len(os.Args) < 2 {
		finish(Fail("Expected at least one argument"))
	}

	switch os.Args[1] {
	case "init":
		finish(plugin.Init())

	case "attach":
		if len(os.Args) != 4 {
			finish(Fail("attach expected exactly 4 arguments; got ", os.Args))
		}

		opt := plugin.NewOptions()
		if err := json.Unmarshal([]byte(os.Args[2]), opt); err != nil {
			finish(Fail("Could not parse options for attach; got ", os.Args[2]))
		}

		nodeName := os.Args[3]

		finish(plugin.Attach(opt, nodeName))

	case "detach":
		if len(os.Args) != 4 {
			finish(Fail("detach expected exactly 3 arguments; got ", os.Args))
		}

		device := os.Args[2]
		finish(plugin.Detach(device, os.Args[3]))

	default:
		finish(NotSupport(os.Args))
	}

}
