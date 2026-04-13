# Context Sources — Track 1

> This file lists the external `.md` files that track 1 depends on.
> Before running prompt 01, copy each source into this `context/` folder
> under the snapshot filename listed below.
>
> Subsequent tracks (`track_1_2` onward) do NOT reference these external files.
> They use snapshots from the prior track's `concept_map.json` and session syntheses only.

---

## Sources to snapshot

| Snapshot filename | Source path (relative to repo root) | Purpose |
|---|---|---|
| `browser_cli_parity.snapshot.md` | `BROWSER_CLI_PARITY.md` | Primary signal for D1/D5: agentic loop design, A2A state machine, sprint backlog |
| `agent_state_machine.snapshot.md` | `agent_state_machine.md` | Deep signal for concept 27 (agentic loop), concept 22 (context window), concept 28 (multi-agent) |
| `debug_usage.snapshot.md` | `features/debug/debug_usage.md` | Signal for concept 26 (tool use), concept 29 (model capability calibration) |

---

## How to snapshot

1. `cp BROWSER_CLI_PARITY.md .experiments/track_1/context/browser_cli_parity.snapshot.md`
2. `cp agent_state_machine.md .experiments/track_1/context/agent_state_machine.snapshot.md`
3. `cp features/debug/debug_usage.md .experiments/track_1/context/debug_usage.snapshot.md`

Run these from the repo root. The snapshots are gitignored (`.experiments/` is gitignored).
They serve as the frozen input for prompt 01 — if the source files change, the snapshots
do not change. This preserves reproducibility.

---

## Snapshot date

Record the date and git commit when you took the snapshots:

| File | Snapshotted at | Git commit |
|---|---|---|
| `browser_cli_parity.snapshot.md` | _(fill when snapshotted)_ | _(fill commit hash)_ |
| `agent_state_machine.snapshot.md` | _(fill when snapshotted)_ | _(fill commit hash)_ |
| `debug_usage.snapshot.md` | _(fill when snapshotted)_ | _(fill commit hash)_ |

---

## Concept coverage by source

| Source | Primary concepts covered |
|---|---|
| `browser_cli_parity.snapshot.md` | 2 (gradient descent — via retry logic), 7 (attention intuition), 22 (context window), 27 (agentic loop), 28 (multi-agent), 29 (model capability calibration) |
| `agent_state_machine.snapshot.md` | 26 (tool use), 27 (agentic loop), 28 (multi-agent), 22 (context window) |
| `debug_usage.snapshot.md` | 26 (tool use), 29 (model capability calibration) |

Note: Concepts 0–13 (ML Fundamentals and ML Math Theory branches) have minimal coverage
in this corpus — this is expected. Prompt 01 will assign bloom_current = 1 with confidence
"low" for these. The first quiz sessions should include more questions from these branches
to establish a real signal.
