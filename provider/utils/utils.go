package utils

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/denverdino/aliyungo/metadata"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
)

// used for global ak
type DefaultOptions struct {
	Global struct {
		KubernetesClusterTag string
		AccessKeyID          string `json:"accessKeyID"`
		AccessKeySecret      string `json:"accessKeySecret"`
		Region               string `json:"region"`
	}
}

const (
	encodedCredPath = "/etc/kubernetes/cloud-config.alicloud"
	credPath        = "/etc/kubernetes/cloud-config"
	USER_AKID       = "/etc/.volumeak/akId"
	USER_AKSECRET   = "/etc/.volumeak/akSecret"
	METADATA_URL    = "http://100.100.100.200/latest/meta-data/"
	REGIONID_TAG    = "region-id"
	INSTANCEID_TAG  = "instance-id"
)

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

func Finish(result Result) {
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

func FinishError(message string) {
	log.Info("Exit with Error: ", message)
	Finish(Fail(message))
}

// Result
type Result struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Device  string `json:"device,omitempty"`
}

// run shell command
func Run(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to run cmd: " + cmd + ", with out: " + string(out) + ", with error: " + err.Error())
	}
	return string(out), nil
}

func CreateDest(dest string) error {
	fi, err := os.Lstat(dest)

	if os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if fi != nil && !fi.IsDir() {
		return fmt.Errorf("%v already exist but it's not a directory", dest)
	}
	return nil
}

func IsMounted(mountPath string) bool {
	cmd := fmt.Sprintf("mount | grep \"%s type\" | grep -v grep", mountPath)
	out, err := Run(cmd)
	if err != nil || out == "" {
		return false
	}
	return true
}

func Umount(mountPath string) bool {
	cmd := fmt.Sprintf("umount -f %s", mountPath)
	_, err := Run(cmd)
	if err != nil {
		return false
	}
	return true
}

// check file exist in volume driver;
func IsFileExisting(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// Get regionid instanceid;
func GetRegionAndInstanceId() (string, string, error) {
	regionId, err := GetMetaData(REGIONID_TAG)
	if err != nil {
		return "", "", err
	}
	instanceId, err := GetMetaData(INSTANCEID_TAG)
	if err != nil {
		return "", "", err
	}
	return regionId, instanceId, nil
}

// get metadata
func GetMetaData(resource string) (string, error) {
	resp, err := http.Get(METADATA_URL + resource)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetRegionIdAndInstanceId(nodeName string) (string, string, error) {
	strs := strings.SplitN(nodeName, ".", 2)
	if len(strs) < 2 {
		return "", "", fmt.Errorf("failed to get regionID and instanceId from nodeName")
	}
	return strs[0], strs[1], nil
}

// save json data to file
func WriteJosnFile(obj interface{}, file string) error {
	maps := make(map[string]interface{})
	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() != "" {
			maps[t.Field(i).Name] = v.Field(i).String()
		}
	}
	rankingsJson, _ := json.Marshal(maps)
	if err := ioutil.WriteFile(file, rankingsJson, 0644); err != nil {
		return err
	}
	return nil
}

// parse json to struct
func ReadJsonFile(file string) (map[string]string, error) {
	jsonObj := map[string]string{}
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &jsonObj)
	if err != nil {
		return nil, err
	}
	return jsonObj, nil
}

// read ossfs ak from local or from secret file
func GetLocalAK() (string, string) {
	accessKeyID, accessSecret := "", ""
	//accessKeyID, accessSecret = GetLocalAK()
	//if accessKeyID == "" || accessSecret == "" {
	if IsFileExisting(USER_AKID) && IsFileExisting(USER_AKSECRET) {
		raw, err := ioutil.ReadFile(USER_AKID)
		if err != nil {
			log.Error("Read User AK ID file error:", err.Error())
			return "", ""
		}
		accessKeyID = string(raw)

		raw, err = ioutil.ReadFile(USER_AKSECRET)
		if err != nil {
			log.Error("Read User AK Secret file error:", err.Error())
			return "", ""
		}
		accessSecret = string(raw)
	} else {
		accessKeyID, accessSecret = GetLocalSystemAK()
	}
	//}
	return strings.TrimSpace(accessKeyID), strings.TrimSpace(accessSecret)
}

// read default ak from local file or from STS
func GetDefaultAK() (string, string, string) {
	accessKeyID, accessSecret := GetLocalAK()

	accessToken := ""
	if accessKeyID == "" || accessSecret == "" {
		accessKeyID, accessSecret, accessToken = GetSTSAK()
	}

	return accessKeyID, accessSecret, accessToken

}

// get STS AK
func GetSTSAK() (string, string, string) {
	m := metadata.NewMetaData(nil)

	rolename := ""
	var err error
	if rolename, err = m.Role(); err != nil {
		log.Fatal("Get role name error: ", err.Error())
		return "", "", ""
	}
	role, err := m.RamRoleToken(rolename)
	if err != nil {
		log.Fatal("Get STS Token error, " + err.Error())
		return "", "", ""
	}
	return role.AccessKeyId, role.AccessKeySecret, role.SecurityToken
}

func GetLocalSystemAK() (string, string) {
	var accessKeyID, accessSecret string
	var defaultOpt DefaultOptions

	if IsFileExisting(encodedCredPath) {
		raw, err := ioutil.ReadFile(encodedCredPath)
		if err != nil {
			FinishError("Read cred file failed: " + err.Error())
		}
		err = json.Unmarshal(raw, &defaultOpt)
		if err != nil {
			FinishError("Parse json cert file error: " + err.Error())
		}
		keyID, err := b64.StdEncoding.DecodeString(defaultOpt.Global.AccessKeyID)
		if err != nil {
			FinishError("Decode accesskeyid failed: " + err.Error())
		}
		secret, err := b64.StdEncoding.DecodeString(defaultOpt.Global.AccessKeySecret)
		if err != nil {
			FinishError("Decode secret failed: " + err.Error())
		}
		accessKeyID = string(keyID)
		accessSecret = string(secret)
	} else if IsFileExisting(credPath) {
		raw, err := ioutil.ReadFile(credPath)
		if err != nil {
			FinishError("Read cred file failed: " + err.Error())
		}
		err = json.Unmarshal(raw, &defaultOpt)
		if err != nil {
			FinishError("New Ecs Client error json, " + err.Error())
		}
		accessKeyID = defaultOpt.Global.AccessKeyID
		accessSecret = defaultOpt.Global.AccessKeySecret

	} else {
		return "", ""
	}
	return accessKeyID, accessSecret
}

// PathExists returns true if the specified path exists.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func IsLikelyNotMountPoint(file string) (bool, error) {
	stat, err := os.Stat(file)
	if err != nil {
		return true, err
	}
	rootStat, err := os.Lstat(file + "/..")
	if err != nil {
		return true, err
	}
	// If the directory has a different device as parent, then it is a mountpoint.
	if stat.Sys().(*syscall.Stat_t).Dev != rootStat.Sys().(*syscall.Stat_t).Dev {
		return false, nil
	}

	return true, nil
}
