# Build Go-based Claude Code hooks. Stow handles symlinking the *-bin
# binaries into ~/.claude/hooks/ via .stow-local-ignore at the package root.
#
# Usage:
#   make           # build all binaries
#   make clean     # remove built binaries

HOOKS := block-aws-cli block-dangerous-git block-dangerous-rm block-env-files enforce-root
BINS  := $(addsuffix -bin,$(HOOKS))

.PHONY: all build clean

all: build

build: $(BINS)

# Each *-bin target rebuilds when any .go file in its source dir changes.
%-bin: %/main.go %/go.mod
	@echo "==> building $@"
	cd $* && go mod tidy >/dev/null 2>&1 && go build -o ../$@ .

clean:
	@rm -f $(BINS)
	@echo "==> cleaned"
