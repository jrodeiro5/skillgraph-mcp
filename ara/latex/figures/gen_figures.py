"""Generate figures for skillgraph-mcp paper."""
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import numpy as np

# ── Figure 1: Scaling — raw vs gateway token cost ──────────────────────────
servers = list(range(1, 11))
# Cumulative tools added smallest-to-largest
tool_counts = [2, 4, 5, 9, 18, 28, 41, 61, 84, 113]
per_tool = 32.4
raw_tokens = [int(t * per_tool) for t in tool_counts]
gateway_tokens = [287] * 10

fig, ax = plt.subplots(figsize=(6, 3.8))

ax.plot(servers, raw_tokens, "o-", color="#d62728", linewidth=2,
        markersize=6, label="Raw loading (all schemas)")
ax.plot(servers, gateway_tokens, "s--", color="#1f77b4", linewidth=2,
        markersize=6, label="skillgraph-mcp (list\\_skills)")

# Crossover annotation
ax.axvline(x=4, color="gray", linestyle=":", linewidth=1.2, alpha=0.7)
ax.annotate("Crossover\n(4 servers, 9 tools)",
            xy=(4, 287), xytext=(5.2, 500),
            fontsize=8, color="gray",
            arrowprops=dict(arrowstyle="->", color="gray", lw=1))

# 92% annotation at 10 servers
ax.annotate("92% reduction\n(3,661 → 287)",
            xy=(10, 3661), xytext=(7.5, 3200),
            fontsize=8, color="#d62728",
            arrowprops=dict(arrowstyle="->", color="#d62728", lw=1))

ax.set_xlabel("Number of downstream MCP servers", fontsize=10)
ax.set_ylabel("Upfront tokens consumed", fontsize=10)
ax.set_title("Token cost: raw loading vs progressive disclosure", fontsize=11)
ax.legend(fontsize=9, loc="upper left")
ax.set_xlim(0.5, 10.5)
ax.set_ylim(0, 4200)
ax.set_xticks(servers)
ax.grid(axis="y", alpha=0.3)
fig.tight_layout()
fig.savefig("figures/fig1_scaling.pdf", bbox_inches="tight")
fig.savefig("figures/fig1_scaling.png", bbox_inches="tight", dpi=150)
print("Figure 1 saved.")

# ── Figure 2: Per-task token cost breakdown ─────────────────────────────────
fig2, ax2 = plt.subplots(figsize=(5.5, 3.2))

scenarios = [
    "Raw\n(all 113 tools)",
    "Gateway\n(task uses 0 skills)",
    "Gateway\n(1 skill: memory)",
    "Gateway\n(1 skill: playwright)",
]
totals   = [3661, 287, 575, 1032]
base     = [3661, 287, 287, 287]
on_demand= [0,    0,  288, 745]

x = np.arange(len(scenarios))
bars1 = ax2.bar(x, base, color=["#d62728", "#1f77b4", "#1f77b4", "#1f77b4"],
                alpha=0.85, label="Upfront (list\\_skills)")
bars2 = ax2.bar(x[2:], on_demand[2:], bottom=base[2:],
                color="#aec7e8", alpha=0.85, label="On-demand (use\\_skill)")

for bar, val in zip(list(bars1) + list(bars2), [3661, 287, 287, 287, 288, 745]):
    pass  # labels below

for i, (total, scenario) in enumerate(zip(totals, scenarios)):
    ax2.text(i, total + 60, f"{total:,}", ha="center", fontsize=8.5, fontweight="bold")

ax2.set_ylabel("Total tokens", fontsize=10)
ax2.set_title("Per-task token cost by scenario", fontsize=11)
ax2.set_xticks(x)
ax2.set_xticklabels(scenarios, fontsize=8.5)
ax2.set_ylim(0, 4400)
ax2.legend(fontsize=9)
ax2.grid(axis="y", alpha=0.3)
fig2.tight_layout()
fig2.savefig("figures/fig2_per_task.pdf", bbox_inches="tight")
fig2.savefig("figures/fig2_per_task.png", bbox_inches="tight", dpi=150)
print("Figure 2 saved.")
