# Changelog

## [1.1.0](https://github.com/jrodeiro5/skillgraph-mcp/compare/v1.0.0...v1.1.0) (2026-07-12)


### Features

* 20+ server scale — keyword fallback, skill isolation, skill:// resources ([0235bce](https://github.com/jrodeiro5/skillgraph-mcp/commit/0235bce21cd7870e106d05a35aae79218df5106a))
* auto-rebuild embed index on graph update ([5fbb31e](https://github.com/jrodeiro5/skillgraph-mcp/commit/5fbb31e95adf4f5f191535880c9aaa29ac6c2ea9))
* **cmd,docs:** generate Claude Code SKILL.md files per downstream skill ([3fd797e](https://github.com/jrodeiro5/skillgraph-mcp/commit/3fd797ee2e056576090100225938dada62af4943))
* **doctor:** check GitHub Releases for available updates ([e11ef21](https://github.com/jrodeiro5/skillgraph-mcp/commit/e11ef21be0abf5a8d40ee63af732f89837f49a00))
* embedding-based tool retrieval + fix skill/MCP confusion ([c0a0346](https://github.com/jrodeiro5/skillgraph-mcp/commit/c0a0346bfbbba1e18a882e09dc7254402139630b))
* **mcpserver:** redirect agents to use_skill when GetServer fails ([886faef](https://github.com/jrodeiro5/skillgraph-mcp/commit/886faef9a1eecd5256e28a63f855b1fe088a5457))
* safe generate-skills + format-compact line collapse + npm readme fix ([9f9b2cd](https://github.com/jrodeiro5/skillgraph-mcp/commit/9f9b2cdb06de5947e19fc05a0763eb2de57ec37d))


### Bug Fixes

* **execute_code:** capture stdout as fallback and preserve partial output on error ([d026c8e](https://github.com/jrodeiro5/skillgraph-mcp/commit/d026c8e5c9142cd31a535ac2327d0dbf52fe2d70))

## [Unreleased](https://github.com/jrodeiro5/skillgraph-mcp/compare/v1.0.0...HEAD)


### Known Limitations

* **execute_code:** remote HTTP MCP servers (`type: http`) and OAuth-gated `mcp-remote` servers block gateway startup — they must complete their handshake before skillgraph responds to the client. Avoid placing remote HTTP servers in `mcp.json`; configure them directly in the MCP client instead.


### Bug Fixes

* **execute_code:** capture stdout from `print()` calls as fallback when code returns `None` — agents no longer need to use `return` explicitly ([`internal/tools/execute_code.go`](internal/tools/execute_code.go))


## [1.0.0](https://github.com/jrodeiro5/skillgraph-mcp/compare/v0.1.1...v1.0.0) (2026-05-29)


### Features

* **cli:** default --lattice-dir to os.UserCacheDir (ADR-0002) ([59a64fd](https://github.com/jrodeiro5/skillgraph-mcp/commit/59a64fd85dcc340aa9ca044bb5634033b5023a47))
* **security:** gate register_server to loopback HTTP (ADR-0001) ([027d762](https://github.com/jrodeiro5/skillgraph-mcp/commit/027d762cc9bf7d3c8503da817a7b4678f6b9708b))


### Bug Fixes

* **execute_code:** write trajectory synchronously to drop t.TempDir race ([c954726](https://github.com/jrodeiro5/skillgraph-mcp/commit/c9547266282f0b66d0f392591686fdfd3f522809))


### Documentation

* **readme:** document API stability surface for v1.0 ([8d8bf0e](https://github.com/jrodeiro5/skillgraph-mcp/commit/8d8bf0ee4712cbad57ac264ec68927ed6bda4059))

## [0.1.1](https://github.com/jrodeiro5/skillgraph-mcp/compare/v0.1.0...v0.1.1) (2026-05-29)


### Bug Fixes

* **mcpserver:** sanitize tool names for execute_code Python sandbox ([3ff6ff2](https://github.com/jrodeiro5/skillgraph-mcp/commit/3ff6ff2cdaa6214235a91f314284d5fdb31f78a8))

## [0.1.0](https://github.com/kurtisvg/skillful-mcp/compare/v0.0.1...v0.1.0) (2026-04-06)


### Features

* add ${VAR} environment variable expansion in mcp.json ([#2](https://github.com/kurtisvg/skillful-mcp/issues/2)) ([be29968](https://github.com/kurtisvg/skillful-mcp/commit/be29968e47b0c660544f0850d838f862253208df))
* add description, allowedTools, and allowedResources options ([#17](https://github.com/kurtisvg/skillful-mcp/issues/17)) ([c5db3b2](https://github.com/kurtisvg/skillful-mcp/commit/c5db3b2355b750991facc0296a53bbc93648d93a))
* add Docker support with multi-platform builds ([#27](https://github.com/kurtisvg/skillful-mcp/issues/27)) ([0dea594](https://github.com/kurtisvg/skillful-mcp/commit/0dea594830f6447485f7b4674747c759c35b6489))
* add README ([637552d](https://github.com/kurtisvg/skillful-mcp/commit/637552d7f1c3a2d5fc1e2033261b03e8c1387575))
* add support for structured output ([#11](https://github.com/kurtisvg/skillful-mcp/issues/11)) ([7e732da](https://github.com/kurtisvg/skillful-mcp/commit/7e732da9e70301bc12e06d3ad8ad28a8fd669a4e))
* deterministic param ordering and type validation for execute_code ([b64dcd2](https://github.com/kurtisvg/skillful-mcp/commit/b64dcd21683fcf7f28dc7179728b99dad9bbd312))
* initial commit of mvp ([ecafd1c](https://github.com/kurtisvg/skillful-mcp/commit/ecafd1ce913cff0a1bfa58a4466b446638ceb285))
* wire downstream tools as callable functions in execute_code ([#4](https://github.com/kurtisvg/skillful-mcp/issues/4)) ([595616b](https://github.com/kurtisvg/skillful-mcp/commit/595616b9c58849df11c8527686f0888961cf876d))


### Bug Fixes

* address misc issues ([#15](https://github.com/kurtisvg/skillful-mcp/issues/15)) ([24bef9e](https://github.com/kurtisvg/skillful-mcp/commit/24bef9e3e68d2a8ba72af10f9533d82632b6de7c))
* consolidate release workflow ([#25](https://github.com/kurtisvg/skillful-mcp/issues/25)) ([fe9ef0f](https://github.com/kurtisvg/skillful-mcp/commit/fe9ef0f28e33d65412093e6492eb6e3a091a17bf))
* startup messages to stderr. (reserve stdio for JSON-RPC MCP messages) ([#1](https://github.com/kurtisvg/skillful-mcp/issues/1)) ([1ce1658](https://github.com/kurtisvg/skillful-mcp/commit/1ce1658d110af051d7296fc12355a64c2593a54a))
