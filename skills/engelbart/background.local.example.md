# Engelbart Local Background

This file is a template for local, non-portable meeting-minutes context. Copy it to `skills/engelbart/background.local.md`; the copied file is ignored by git.

## Team

- Team: {team name}
- Lead: {name}
- Members: {name list}

## Products And Services

- {service name}: {short description}

## Term Hints

- `{transcript variant}` -> `{canonical term}`. Reason: {why this is likely}.
- Product/service names listed above are strong correction candidates for phonetically similar transcript variants. Example: if `팅글` is listed, `킹글 스테이징` should resolve to `팅글 staging` when the context is service staging migration.

## Speaker Hints

- `{speaker label}` -> `{name or role}`. Evidence rule: use only when the transcript, this local background file, or user-provided context supports the mapping.

## Confirmed Roles

- `{name}`: `{role}`. If a role is confirmed here, do not add `추정` when using that role in owner, metadata, or action fields.

## Aliases

- `{alias}`: `{name}`. Apply explicit aliases consistently in decisions, actions, topic summaries, and correction maps.
- Standalone Korean honorific-like words can be name misrecognitions when they phonetically match one roster member. Example: `프로님` may be an alias for `{푸름님}` when `{이푸름}` is in the roster. But `{name} 프로님` should be treated as a title/honorific unless the local file explicitly says otherwise.
