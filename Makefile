export GO15VENDOREXPERIMENT=1

BUILD_DIR ?= ./out

.PHONY: driver
frakti: $(shell $(LOCALKUBEFILES))
	go build -a -o ${BUILD_DIR}/disk ./provider/alicloud

.PHONY: install
install:
	mkdir -p /usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk
	cp -f ./out/disk /usr/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk

clean:
	rm -rf ${BUILD_DIR}
