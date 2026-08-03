# #258 Orca owner sealed-claim priority

## Goal

Ensure a dispatched Orca owner claims its sealed generation instead of invoking
the coordinator-only recovery resume command. Make a duplicate same-generation
resume observably idempotent instead of looking like a fresh launch.

## Evidence

- Lifecycle `io-268bd6ac6e7a` generation 2 launched Orca Run
  `run_b2a26ca3655b` and task `task_11279cb7c785`.
- The owner prompt says both to execute the injected sealed claim and to execute
  status' recovery `execution resume`; the live owner selected resume and exited.
- `PlanResume` already returns `existing_binding` when the same-generation task
  and terminal are live, but that disposition is dropped before the public CLI
  response. The response therefore looks like an undifferentiated success.

## Implementation

1. RED: assert that the owner prompt names the injected sealed claim as the only
   owner action and forbids a dispatched owner from running coordinator resume.
2. RED: assert that a same-generation live-owner resume returns an explicit
   `existing_binding` disposition without allocating an operation or invoking a
   launch stage.
3. GREEN: rewrite the prompt priority rule in the embedded template and its
   byte-identical Karpathy source. Preserve the direct-mode `claim=none` path.
4. GREEN: carry the domain resume disposition through the application result,
   inbound adapter, CLI JSON, and MCP response without changing the lease or
   external-effect state machine.
5. VERIFY: run targeted prompt/domain/application/CLI/MCP tests, response
   contract goldens, full tests, race, vet, build, and architecture checks.
6. DOGFOOD: merge and install the exact #258 head, reseed #248 to a newly sealed
   generation, resume it through Orca, and verify claim, branch-link recording,
   and the first governed production mutation.

## Safety boundary

- `execution resume` remains a coordinator recovery operation; genuine dead
  owner recovery continues to reuse or create a terminal through the existing
  inventory decision table.
- Same-generation live-owner resume remains idempotent and side-effect free, but
  is explicitly labeled `existing_binding` and returns the sealed claim command.
- No claim token content is copied into prompts, reports, tests, or remote data.
- GitHub/GitLab identity checks and direct-mode ownership are unchanged.
