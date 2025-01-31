HOSTNAME=nullplatform
NAMESPACE=com
NAME=nullplatform
BINARY=terraform-provider-${NAME}
VERSION=0.0.15
TEST := ./...

OS := $(shell uname -o | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
OS_ARCH := $(OS)_$(ARCH)

default: install

build:
	go build -o ${BINARY}

release:
	goreleaser release --rm-dist --snapshot --skip-publish  --skip-sign

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test: 
	go test -i $(TEST) || exit 1                                                   
	echo $(TEST) | xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4                    

testacc: 
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m   

update-docs:
	tfplugindocs generate -provider-name nullplatform --rendered-provider-name "nullplatform"
