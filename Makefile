HOSTNAME=nullplatform
NAMESPACE=com
NAME=nullplatform
BINARY=terraform-provider-${NAME}
VERSION=0.0.72
TEST := ./...

OS := $(shell uname -o | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
OS_ARCH := $(OS)_$(ARCH)

default: install

build:
	go build -o ${BINARY}

release:
	goreleaser release --rm-dist --snapshot --skip-publish  --skip-sign

debug: build
	go install .

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test:
	go test -i $(TEST) || exit 1
	echo $(TEST) | xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4

# Unit tests with a coverage profile, plus the two human views of it:
# a per-function summary on stdout and an annotated-source HTML report.
test-coverage:
	go test ./nullplatform/ -count=1 -coverprofile=coverage.out -timeout=120s
	go tool cover -func=coverage.out | tail -20
	go tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html for the annotated source"

# The coverage gate (tools/covergate): every line this branch added (vs BASE,
# default origin/main) must be executed by some test, and the repo total must
# not fall below scripts/coverage_floor.txt. Fails otherwise — the same gate
# CI enforces.
BASE ?= origin/main
coverage-new: test-coverage
	go run ./tools/covergate -base $(BASE)

testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

update-docs:
	tfplugindocs generate -provider-name nullplatform --rendered-provider-name "nullplatform"
