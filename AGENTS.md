# AGENTS.md — MicroCloud Agent Instructions

MicroCloud is an open source cloud platform written in Go. It orchestrates a cluster of machines by auto-configuring LXD, MicroCeph, and MicroOVN. Module: `github.com/canonical/microcloud/microcloud`.

## Prerequisites

MicroCloud requires Go 1.26.2 or higher.

- CGO native dependency: `dqlite`. Fetch and build it once with:

  ```
  make deps
  ```

  This clones and builds the `dqlite` C library under `$GOPATH/deps/dqlite`. Set `CGO_CFLAGS`, `CGO_LDFLAGS`, and `LD_LIBRARY_PATH` accordingly if building outside of `make`.

## Repository layout

```
api/            HTTP API handlers and request/response types
  types/        Shared API type definitions
client/         Go client library for the MicroCloud API
cmd/
  microcloud/   CLI binary
  microcloudd/  Daemon binary
  tui/          Terminal UI library (tables, prompts, autocomplete)
database/       dqlite schema and CRUD helpers
multicast/      UDP multicast peer discovery
service/        Service interface and wrappers for LXD, MicroCeph, MicroOVN
test/
  suites/       System test suites (bash)
  includes/     Shared shell helper functions
  lint/         Shell lint scripts
  e2e/          Post-deployment end-to-end tests (Terraform)
version/        Single source of truth for the version string
doc/            Sphinx documentation
```

### Auto-generated files — do not edit manually

Update these via the listed `make` target instead of editing by hand:

| File | Command |
| --- | --- |
| `go.mod`, `go.sum` | `make update-gomod` |

## Build

```sh
# Production build
make build

# Test build (scripted TUI input, simplified wordlist)
make build-test
```

## Validate before committing

Run these in order. Each must pass before moving to the next.

```sh
# 1. Static analysis (golangci-lint, revive, shell lint scripts)
make check-static

# 2. Unit tests
make check-unit

# 3. Full build
make build
```

`make check-static` runs `golangci-lint`, `revive`, and the shell scripts under `test/lint/`. Review any reformatted files and stage only changes relevant to your work.

## Key conventions

### Commit format

```
<component/subcomponent>: <concise change description>
```

Examples:
- `api/services: Use the authHandlerMTLS func`
- `cmd/microcloud: Remove token add command`
- `service/lxd: Fix storage pool bootstrap error handling`

Use separate commits for each logical change and for changes to different components. See `CONTRIBUTING.md` for DCO sign-off (`git commit -s`) and GPG signature requirements.

### Error messages

- Use `"Cannot"` not `"Unable to"`.
- Capitalize the first letter of error strings: `fmt.Errorf("Cannot connect to ...")`.
- No contractions: `"does not"` not `"doesn't"`.
- US English spelling throughout (`behavior`, `color`, `initialize`, `organization`).

### Go code style

- No inline variable declarations inside `if` conditions — assign on a separate line first.
- Prefer early returns to reduce nesting.
- Import grouping (enforced by `gci`): stdlib → external → `github.com/canonical/microcloud/microcloud`.
- Check `service/` for existing helpers before implementing utilities from scratch.
- Both the `microcloud` CLI and `microcloudd` daemon enforce `os.Geteuid() == 0`; keep this behavior.

### Shell test style

- Use `jq --exit-status` (`jq -e`) when asserting field presence or values.
- For expected command failure: `if cmd_should_fail; then echo "ERROR: ..."; exit 1; fi`
- Use helper functions from `test/includes/microcloud.sh` (`validate_system_*`, `reset_systems`, etc.) rather than reimplementing validation logic.

### Build tags

| Tag | Purpose |
| --- | --- |
| `agent` | Production build |
| `test` | Enables `TEST_CONSOLE=1` scripted input and replaces the EFF wordlist with a small test wordlist |
