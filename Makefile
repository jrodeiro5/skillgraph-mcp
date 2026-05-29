.PHONY: build test lint install inspect inspect-downstream ci ci-release

BINARY      := skillgraph-mcp
INSTALL_DIR := $(HOME)/.local/bin
CONFIG      ?= $(HOME)/javi/agent-hub/config/skillgraph-config.json
LATTICE_DIR ?= $(HOME)/.mcp_lattice

build:
	go build -o $(BINARY) .

test:
	go test -timeout 120s ./...

lint:
	golangci-lint run

install: build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed at $(INSTALL_DIR)/$(BINARY)"

# Open MCP Inspector UI against the locally-installed skillgraph-mcp.
# Requires Node (Inspector is a TypeScript app, npx-run only). The gateway
# itself stays pure Go — Inspector talks to it via stdio JSON-RPC like any
# other MCP client.
#   make inspect                       # default config
#   make inspect CONFIG=/path/to/mcp.json
inspect:
	npx -y @modelcontextprotocol/inspector -- \
	  $(INSTALL_DIR)/$(BINARY) \
	  --config $(CONFIG) \
	  --lattice-dir $(LATTICE_DIR)

# Debug a single downstream MCP in isolation, bypassing skillgraph-mcp.
# Useful when you suspect the gateway is masking a downstream issue
# (e.g. the gitnexus EOF that took the whole gateway down).
#   make inspect-downstream CMD="npx -y @brave/brave-search-mcp-server@latest"
#   make inspect-downstream CMD="$(HOME)/.local/share/pnpm/bin/gitnexus mcp"
inspect-downstream:
	@if [ -z "$(CMD)" ]; then echo "usage: make inspect-downstream CMD='<cmd> [args...]'"; exit 1; fi
	npx -y @modelcontextprotocol/inspector $(CMD)

# Run the test workflow locally via act (nektos/act). Catches workflow
# regressions before pushing to GitHub. Requires Docker. First run pulls
# a ~600MB runner image; subsequent runs are cached.
#
# GITHUB_TOKEN must be exported in the shell — act uses it to clone the
# actions referenced in `uses:` (setup-go, github-script, etc.). Without
# it git clone fails because GitHub disabled anonymous Password auth.
#   make ci
ci:
	@test -n "$$GITHUB_TOKEN" || (echo "GITHUB_TOKEN not set (act needs it to clone actions/*)"; exit 1)
	act push -W .github/workflows/test.yml -s GITHUB_TOKEN=$$GITHUB_TOKEN

# Simulate the release workflow_dispatch locally. Override TAG to test
# against a specific version; defaults to v0.0.0-test so nothing collides
# with a real release if you happen to push artifacts by accident.
#   make ci-release
#   make ci-release TAG=v0.1.1
TAG ?= v0.0.0-test
ci-release:
	@test -n "$$GITHUB_TOKEN" || (echo "GITHUB_TOKEN not set (act needs it to clone actions/*)"; exit 1)
	act workflow_dispatch -W .github/workflows/release-please.yml --input tag=$(TAG) -s GITHUB_TOKEN=$$GITHUB_TOKEN
