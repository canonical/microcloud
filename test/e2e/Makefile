GOCOVERDIR ?= $(shell go env GOCOVERDIR)

.PHONY: default
default: help

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  e2e            Run the E2E suite against a deployed cluster"
	@echo "  check-unit     Run unit tests"
	@echo "  check-static   Run golangci-lint and revive"
	
# Run the ginkgo e2e suite directly against an already-deployed MicroCloud
# cluster. Does not provision anything: REMOTE (and, for SSH-based
# testbeds, E2E_TESTBED_FILE) must already be set by the caller.
.PHONY: e2e
e2e:
	go run github.com/onsi/ginkgo/v2/ginkgo -v $(ARGS) --junit-report=report.xml .

.PHONY: check-unit
check-unit:
ifeq "$(GOCOVERDIR)" ""
	go test ./framework/...
else
	go test ./framework/... -cover -test.gocoverdir="${GOCOVERDIR}"
endif

.PHONY: check-static
check-static:
	golangci-lint run --timeout 5m -c ../../.golangci.yaml
	revive -config ../../revive.toml -set_exit_status ./...

