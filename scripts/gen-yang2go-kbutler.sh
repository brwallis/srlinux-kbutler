#!/bin/bash
YANG_SRC_PATH="../cmd/kbutler/yang"
GO_OUT_PATH="../internal/kbutler/yang"

echo "Generate Go bindings for SR Linux Kubernetes Butler YANG modules"
go mod download

YGOT_DIR=`go list -f '{{ .Dir }}' -m github.com/openconfig/ygot`

mkdir -p ${GO_OUT_PATH}
go run $YGOT_DIR/generator/generator.go \
   -path=${YANG_SRC_PATH}/ -output_file=${GO_OUT_PATH}/model.go -package_name=kbutler_yang -generate_fakeroot \
   ${YANG_SRC_PATH}/kbutler.yang

go mod tidy
