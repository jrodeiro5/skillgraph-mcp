.PHONY: build test lint install inspect inspect-downstream

BINARY      := skillgraph-mcp
INSTALL_DIR := $(HOME)/.local/bin
CONFIG      ?= $(HOME)/javi/agent-hub/config/skillful-config.json
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
	npx -y @modelcontextprotocol/inspector \
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
