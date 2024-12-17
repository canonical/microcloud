.PHONY: default
default: build

# Build targets.
.PHONY: build
build:
	go install -tags=agent -v ./cmd/microcloud
	go install -tags=agent -v ./cmd/microcloudd

# Testing targets.
.PHONY: check
check: check-static check-unit check-system

.PHONY: check-unit
check-unit:
	go test ./...

.PHONY: check-system
check-system:
	cd test && ./main.sh

.PHONY: check-unit
check-unit:
	go test ./... -tags test

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

	# Grab MicroOVN from the corresponding support branch.
	go get github.com/canonical/microovn/microovn@branch-22.03

	# There is a reef branch, but its microcluster dependency has breaking changes.
	# Also, the quincy branch has an outdated client for the MicroCeph API offered with reef/stable.
	#
	# As such, we have to grab the latest commit which uses a compatible microcluster,
	# but recent enough to also account for reef's breaking API changes.
	go get github.com/canonical/microceph/microceph@v0.0.0-20240610102219-bc3fe1e7b68f

	# Branches named `vN` where N is a number are reserved by go for module versions,
	# So we are forced to grab the corresponding commit hash for the `v1` branch instead.
	go get github.com/canonical/microcluster@v0.0.0-20241212191815-dba2a1b44d4d

	go mod tidy -go=$(GOMIN)

# Update lxd-generate generated database helpers.
.PHONY: update-schema
update-schema:
	go generate ./...
	gofmt -s -w ./database/
	goimports -w ./database/
	@echo "Code generation completed"

