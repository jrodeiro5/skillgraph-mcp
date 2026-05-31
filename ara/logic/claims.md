# Claims

## C01: Progressive disclosure reduces upfront token cost by ~92%
- **Statement**: Replacing direct MCP server registration with a skillgraph-mcp gateway reduces upfront context token consumption from ~3,660 to 287 tokens in a 10-server, 113-tool deployment.
- **Status**: supported
- **Provenance**: ai-executed (measured empirically this session)
- **Falsification criteria**: A deployment where `list_skills` output exceeds raw schema cost (would require unusually verbose gateway descriptions).
- **Proof**: [T01 — evidence/tables/T01-token-measurements.md]
- **Dependencies**: []
- **Tags**: token-efficiency, progressive-disclosure, empirical

## C02: Token cost scales linearly with raw server count; gateway cost stays constant
- **Statement**: Each additional MCP server adds ~32 tokens/tool to raw loading cost; gateway upfront cost (287 tokens) is invariant to server count.
- **Status**: supported
- **Provenance**: ai-executed (modeled from empirical per-tool rate)
- **Falsification criteria**: Measure gateway `list_skills` output growing proportionally with server count.
- **Proof**: [T02 — evidence/tables/T02-scaling.md]
- **Dependencies**: [C01]
- **Tags**: scaling, token-efficiency

## C03: SkillOpt improves tool routing accuracy over time without manual annotation
- **Statement**: The trace-driven LLM refinement loop produces measurably better tool descriptions and graph relations after N agent interactions than the bootstrap descriptions.
- **Status**: hypothesis
- **Provenance**: ai-suggested
- **Falsification criteria**: Blind evaluation of descriptions pre/post SkillOpt shows no statistically significant improvement.
- **Proof**: [pending — needs before/after evaluation]
- **Dependencies**: []
- **Tags**: skillopt, self-improvement, annotation-free
