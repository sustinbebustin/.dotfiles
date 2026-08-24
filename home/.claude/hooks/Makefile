# Build the Go hook binary. Stow symlinks claude-hooks-bin into
# ~/.claude/hooks/; the sources and build tooling are held back via
# .stow-local-ignore at the package root.
#
# Usage:
#   make            # build the binary
#   make test       # run the test suite
#   make golden     # regenerate the decision corpus (review the diff!)
#   make list       # print the registered rules and the settings.json matcher
#   make clean      # remove the built binary

BIN := claude-hooks-bin
PKG := ./cmd/claude-hooks

# Every rule is compiled into this one binary, so any source change rebuilds it.
SRC := go.mod go.sum $(shell find cmd internal -name '*.go' -not -name '*_test.go')

.PHONY: all build test golden list clean

all: build

build: $(BIN)

$(BIN): $(SRC)
	@echo "==> building $@"
	go build -o $@ $(PKG)

test:
	@echo "==> testing"
	go test ./...

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

# The "matcher" printed here is what the single PreToolUse entry in
# settings.json must be registered with, or a rule silently stops running.
list: $(BIN)
	@./$(BIN) list

# The binary this removes is the live one: ~/.claude/hooks/ holds a stow symlink
# to it, so the guards are unregistered until the next `make build`.
clean:
	@rm -f $(BIN)
	@echo "==> cleaned -- run \`make build\` to restore the guards"
