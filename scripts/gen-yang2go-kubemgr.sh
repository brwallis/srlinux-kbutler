#!/bin/bash
YANG_SRC_PATH="../cmd/kubemgr/yang"
GO_OUT_PATH="../internal/kubemgr/yang"

echo "Generate Go bindings for SR Linux Kube Mgr YANG modules"
go mod download

YGOT_DIR=`go list -f '{{ .Dir }}' -m github.com/openconfig/ygot`

mkdir -p ${GO_OUT_PATH}
go run $YGOT_DIR/generator/generator.go \
   -path=${YANG_SRC_PATH}/ -output_file=${GO_OUT_PATH}/model.go -package_name=kubemgr_yang -generate_fakeroot \
   ${YANG_SRC_PATH}/kube.yang

go mod tidy
