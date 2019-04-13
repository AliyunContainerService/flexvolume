package disk

import "testing"

func TestGetDevicePath(t *testing.T) {

	before := []string{}
	after  := []string{}
	before = append(before, "/dev/vdb", "/dev/vdc")
	after = append(after, "/dev/vdb", "/dev/vdc", "/dev/vdd")
	devices := getDevicePath(before, after)
	t.Log(devices)
}