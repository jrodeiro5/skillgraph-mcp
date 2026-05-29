# ADR-0001: Gate `register_server` to loopback HTTP

## Status

Accepted

## Context

skillgraph-mcp exposes two transports:

- `stdio` (the default): stdin/stdout JSON-RPC, single MCP client per process.
- `http`: an unauthenticated JSON-RPC endpoint, intended for the MCP Inspector,
  the `validate` workflow, and ad-hoc debugging.

The `register_server` gateway tool hot-registers a new downstream MCP server
at runtime. Its config block contains a `command` field, which the gateway
`exec`s as a child process under the gateway's UID. Combined with HTTP
transport bound to a non-loopback address, **any client that can reach the
port can register an arbitrary command and gain RCE as the gateway's user**.

`--host` already defaults to `localhost`, so the typical local-only use case
is safe. The risk is a developer typing `--transport http --host 0.0.0.0` on
a laptop attached to a shared network without realising that
`register_server` is now an attack surface.

## Decision

The `register_server` gateway tool **refuses to register** when the gateway
is serving HTTP and the bind host is not a loopback address. The tool returns
an error result with a message pointing to this ADR and to the Gotchas
section of the README.

Loopback is detected by resolving `--host` and checking whether all addresses
fall inside `127.0.0.0/8` (IPv4) or `::1` (IPv6). `localhost` and `127.0.0.1`
both resolve as loopback; `0.0.0.0`, external IPs, and DNS names that resolve
outside the loopback range do not.

The `--host` flag default remains `localhost`. No behavioural change for
existing users who do not explicitly bind to a non-loopback address.

## Alternatives considered

- **Status quo + Gotcha in README.** Rejected: target audience (a) users
  overwhelmingly do not read security caveats before trying flags, and the
  cost of getting it wrong is RCE. A documentation-only fix is the
  documentation equivalent of storing default DB passwords in a comment.

- **Build-tag for `register_server` (omit from default builds).** Rejected:
  the primary case for target (a) is exactly that an agent can add a new MCP
  on the fly during a session — `register_server` is the mechanism. Stripping
  it from the default binary breaks the use case to mitigate a misuse.

- **Authentication on the HTTP transport.** Out of scope for v0.1.x: real
  authn means token storage, header parsing, replay protection, CSRF on the
  Inspector path, and an operational story for rotation. Single-user tool
  doesn't justify it. Users who need a public HTTP endpoint should put an
  auth proxy in front (caddy, oauth2-proxy, Cloudflare Access, etc.) and not
  expose `register_server`.

- **Change `--host` default to `127.0.0.1` literal.** Rejected as
  unnecessary. `localhost` already resolves to a loopback address in any
  reasonable environment; the cosmetic switch was sold by an earlier draft
  of this ADR as a security win, which it is not. Industry baselines (e.g.
  PostgreSQL's `listen_addresses = 'localhost'`) use the same convention.

## Consequences

- `validate` and Inspector workflows continue to work (they target loopback).
- Existing setups that intentionally serve HTTP on a public interface keep
  working **for read-only and SkillOpt-related tools**, but `register_server`
  now errors out on those setups. That is the desired outcome — those users
  must either move to stdio, put auth in front of the HTTP endpoint, or
  accept the loss of dynamic registration for their public deployment.
- Future "admin" tools (e.g. a hypothetical `delete_server`, `rotate_keys`)
  follow the same gating pattern.
