---
name: cautions/lessons/2026-08-21-api-doc-route-block-assembly-and-evidence-bundling.md
description: Dated lesson — api-doc static gate silently skipped multiline-decorator routes, and review prompts lacked service error evidence; found by dogfooding on 2026-08-21.
---

# 2026-08-21 — api-doc dogfood: multiline decorator routes bypassed static checks; review input had no error-contract evidence

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: api-doc gate dogfooding against the NestJS microservice fixture in `cmd/harness/apidoc/dogfood` (2026-08-21)
- Summary: Two recall failures the api-doc gates shipped with; both now locked by the dogfood ground truth.

## 1. Line-prefix walk-up silently skipped routes with multiline decorator objects

`CheckNestController` assembled each route's decorator block by walking upward
over lines that were blank, `@`-prefixed, `.`-prefixed, or contained `})`. A
multiline `@ApiOperation({ summary: ..., description: '### ...' })` puts
`summary:`/`description:` property lines inside the walk, so the walk stopped
there and the `@Get(...)` decorator never entered the block — `nestRouteRe`
found no route and the whole method was skipped. Controllers that followed the
documented preferred style (sectioned multiline descriptions) were the least
checked. Fix: paren/brace-balance walk (`controllerDecoratorStart`) that
crosses decorator object interiors and stops at member/class boundaries.

## 2. Review prompts carried only the staged diff, so service-layer error contracts were invisible

`api-doc review` bundled the diff of API candidate files, but service files
(`users.service.ts`) never match the candidate regex, so `throw new
NotFoundException` sites — and `ClientProxy.send` → `@MessagePattern` hops with
their `@Catch(RpcException)` filter mappings — could not be seen by the host
agent reviewing the rendered prompt. The 404/403/409 rule existed in the prompt
text with no data behind it. Fix: `reviewfiles.Evidence` extracts throw sites,
microservice hops, and filter mappings and renders them as a "Business Logic
Error Contract Evidence" prompt section; the review rules now require
cross-checking it. Evidence is skipped when `--result` is provided (verdict
ingestion needs no prompt).

## Standard that guards both

`cmd/harness/apidoc/dogfood` materializes a two-service NestJS fixture with a
seeded ground truth (S1–S9 static, E1–E5 review evidence) plus a clean control
fixture: 100% recall on dirty, zero violations on clean. Any gate change must
keep both; new miss patterns get a new seed.
