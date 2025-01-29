GOMIN=1.22.7
GOCOVERDIR ?= $(shell go env GOCOVERDIR)
GOPATH ?= $(shell go env GOPATH)
DQLITE_PATH=$(GOPATH)/deps/dqlite
DQLITE_BRANCH=lts-1.17.x

.PHONY: default
default: build

# Build dependencies
.PHONY: deps
deps:
	# dqlite (+raft)
	@if [ ! -e "$(DQLITE_PATH)" ]; then \
		echo "Retrieving dqlite from ${DQLITE_BRANCH} branch"; \
		git clone --depth=1 --branch "${DQLITE_BRANCH}" "https://github.com/canonical/dqlite" "$(DQLITE_PATH)"; \
	elif [ -e "$(DQLITE_PATH)/.git" ]; then \
		echo "Updating existing dqlite branch"; \
		cd "$(DQLITE_PATH)"; git pull; \
	fi

	cd "$(DQLITE_PATH)" && \
		autoreconf -i && \
		./configure --enable-build-raft && \
		make

# Build targets.
.PHONY: build
build:
ifeq "$(GOCOVERDIR)" ""
	go install -tags=agent -v ./cmd/microcloud
	go install -tags=agent -v ./cmd/microcloudd
else
	go install -tags=agent -v -cover ./cmd/microcloud
	go install -tags=agent -v -cover ./cmd/microcloudd
endif

# Build MicroCloud for testing. Replaces EFF word-list,
# and enables feeding input to questions from a file with TEST_CONSOLE=1.
.PHONY: build-test
build-test:
ifeq "$(GOCOVERDIR)" ""
	go install -tags=test -v ./cmd/microcloud
	go install -tags=test -v ./cmd/microcloudd
else
	go install -tags=test -v -cover ./cmd/microcloud
	go install -tags=test -v -cover ./cmd/microcloudd
endif

# Testing targets.
.PHONY: check
check: check-static check-unit check-system

.PHONY: check-unit
check-unit:
ifeq "$(GOCOVERDIR)" ""
	go test ./...
else
	go test ./... -cover -test.gocoverdir="${GOCOVERDIR}"
endif

.PHONY: check-system
check-system:
	cd test && ./main.sh

.PHONY: check-static
check-static:
ifeq ($(shell command -v golangci-lint 2> /dev/null),)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$HOME/go/bin
endif
ifeq ($(shell command -v revive 2> /dev/null),)
	go install github.com/mgechev/revive@latest
endif
	golangci-lint run --timeout 5m
	revive -config revive.toml -exclude ./cmd/... -set_exit_status ./...
	run-parts --exit-on-error --regex '.sh' test/lint

# Update targets.
.PHONY: update-gomod
update-gomod:
	go get -t -v -u ./...

	# Static pins
	go get github.com/canonical/lxd@stable-5.21 # Stay on v2 dqlite and LXD LTS client
	go get github.com/canonical/microceph@1200ba77f2320be2acec45939f4b96a8ac4f0722 # Right after releasing squid LTS.
	go get github.com/canonical/microovn@branch-24.03 # 24.03 LTS.

	go mod tidy -go=$(GOMIN)

# Update lxd-generate generated database helpers.
.PHONY: update-schema
update-schema:
	go generate ./...
	gofmt -s -w ./database/
	goimports -w ./database/
	@echo "Code generation completed"

doc-%:
	cd doc && $(MAKE) -f Makefile $* ALLFILES='*.md **/*.md'

doc: doc-clean-doc doc-html
