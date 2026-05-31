# T02: Token Cost Scaling — Raw vs Gateway as Servers Added

**Captured:** 2026-05-31  
**Method:** Empirical per-tool rate (32.4 tokens/tool from playwright) applied to cumulative tool counts.  
**Gateway cost:** 287 tokens (constant — measured from list_skills output).  
**Server order:** smallest to largest by tool count (worst-case ordering for gateway).

| Servers | Cumulative tools | Raw tokens | Gateway tokens | Reduction |
|---------|-----------------|------------|----------------|-----------|
| 1       | 2               | 64         | 287            | −348% (gateway costs more) |
| 2       | 4               | 129        | 287            | −122% |
| 3       | 5               | 162        | 287            | −77% |
| **4**   | **9**           | **291**    | **287**        | **≈0% (crossover)** |
| 5       | 18              | 583        | 287            | 51% |
| 6       | 28              | 907        | 287            | 68% |
| 7       | 41              | 1,328      | 287            | 78% |
| 8       | 61              | 1,976      | 287            | 85% |
| 9       | 84              | 2,721      | 287            | 89% |
| **10**  | **113**         | **3,661**  | **287**        | **92%** |

## Key finding

Crossover occurs at ~4 servers / 9 tools. Beyond that, every additional server
increases raw cost linearly while gateway cost stays constant. The gateway is
**not beneficial for 1–3 small servers** — this is an honest limitation to state
in the paper.
