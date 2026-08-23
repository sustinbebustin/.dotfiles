# Build the Go hooks. Stow symlinks the *-bin binaries into ~/.claude/hooks/;
# the sources and build tooling are held back via .stow-local-ignore at the
# package root.
#
# Usage:
#   make           # build all binaries
#   make test      # run the test suite
#   make clean     # remove built binaries

HOOKS := block-aws-cli block-dangerous-git block-dangerous-rm block-env-files enforce-root
BINS  := $(addsuffix -bin,$(HOOKS))

# All hooks live in one module and share internal/hookio, so a change to the
# shared package has to rebuild every binary, not just the one whose main.go
# changed.
SHARED := go.mod go.sum $(wildcard internal/hookio/*.go)

.PHONY: all build test clean

all: build

build: $(BINS)

%-bin: %/main.go $(SHARED)
	@echo "==> building $@"
	go build -o $@ ./$*

test:
	go test ./...

clean:
	@rm -f $(BINS)
	@echo "==> cleaned"
