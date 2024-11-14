GOMIN=1.22.7
GOCOVERDIR ?= $(shell go env GOCOVERDIR)

.PHONY: default
default: build

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
