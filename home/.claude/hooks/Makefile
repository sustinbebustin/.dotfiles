# Build and check the Go hook binary. Stow symlinks claude-hooks-bin into
# ~/.claude/hooks/; the sources and build tooling are held back via
# .stow-local-ignore at the package root.
#
# Usage:
#   make            # build the binary
#   make check      # the pre-commit gate: formatting, linters, tests
#   make fmt        # rewrite files with gofumpt + gci
#   make lint       # golangci-lint run (includes go vet)
#   make lint-fix   # lint, applying the fixes the linters can make themselves
#   make test       # run the test suite
#   make test-race  # run it again under the race detector
#   make cover      # test with coverage, print the per-package summary
#   make golden     # regenerate the decision corpus (review the diff!)
#   make tidy       # go mod tidy
#   make list       # print the registered rules and the settings.json matcher
#   make clean      # remove the built binary and coverage output

BIN := claude-hooks-bin
PKG := ./cmd/claude-hooks
COVER := coverage.out
GOLANGCI ?= golangci-lint

# Every rule is compiled into this one binary, so any source change rebuilds it.
SRC := go.mod go.sum $(shell find cmd internal -name '*.go' -not -name '*_test.go')

.PHONY: all build check fmt fmt-check lint lint-fix test test-race cover golden \
	tidy list clean require-golangci

all: build

build: $(BIN)

$(BIN): $(SRC)
	@echo "==> building $@"
	go build -o $@ $(PKG)

# The gate to clear before committing. fmt-check rather than fmt, so an
# unformatted tree is reported instead of being silently rewritten underfoot.
check: fmt-check lint test

fmt: require-golangci
	@echo "==> formatting"
	@$(GOLANGCI) fmt

# Exits non-zero and prints the diff when anything is unformatted.
fmt-check: require-golangci
	@echo "==> checking formatting"
	@$(GOLANGCI) fmt --diff

lint: require-golangci
	@echo "==> linting"
	@$(GOLANGCI) run

lint-fix: require-golangci
	@echo "==> linting (applying fixes)"
	@$(GOLANGCI) run --fix

test:
	@echo "==> testing"
	go test ./...

test-race:
	@echo "==> testing (race detector)"
	go test -race ./...

cover:
	@echo "==> testing with coverage"
	@go test -coverprofile=$(COVER) ./...
	@go tool cover -func=$(COVER) | tail -1
	@echo "==> full report: go tool cover -html=$(COVER)"

# Rewrites testdata/decisions.json from what the rules decide *now*. The corpus
# is what pins hook behaviour, so a diff here is a behaviour change: read it
# before staging it.
#
# The second run is not redundant. `-run TestDecisions -update` rewrites the
# file and returns early, so the corpus invariants -- every decision exercised,
# every block carrying a reason, no duplicate names -- never see what was just
# written. Without it a regression that flattened every rule to allow would let
# this target report success.
golden:
	@echo "==> regenerating the decision corpus"
	go test ./internal/rules -run TestDecisions -update -count=1
	@echo "==> checking the regenerated corpus"
	@go test ./internal/rules -count=1
	@git --no-pager diff -- internal/rules/testdata/decisions.json

tidy:
	@echo "==> tidying modules"
	go mod tidy

# The "matcher" printed here is what the single PreToolUse entry in
# settings.json must be registered with, or a rule silently stops running.
list: $(BIN)
	@./$(BIN) list

# The binary this removes is the live one: ~/.claude/hooks/ holds a stow symlink
# to it, so the guards are unregistered until the next `make build`.
clean:
	@rm -f $(BIN) $(COVER)
	@echo "==> cleaned -- run \`make build\` to restore the guards"

# golangci-lint supplies both the linters and the formatters. Without this
# check, a missing binary would fail with a bare "command not found" -- or,
# worse, `make check` would look like it had verified something.
require-golangci:
	@command -v $(GOLANGCI) >/dev/null 2>&1 || { \
		echo "==> $(GOLANGCI) not found"; \
		echo "    It runs the linters and the formatters configured in .golangci.yml."; \
		echo "    Install it with: brew install golangci-lint"; \
		exit 1; \
	}
