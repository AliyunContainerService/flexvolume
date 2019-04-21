package oss

import "testing"

func TestCheckOptions(t *testing.T) {
	plugin := &OssPlugin{}
	optin := &OssOptions{Bucket: "aliyun", Url: "oss-cn-hangzhou.aliyuncs.com", OtherOpts: "-o max_stat_cache_size=0 -o allow_other", AkId: "1223455", AkSecret: "22334567"}
	plugin.checkOptions(optin)
}
