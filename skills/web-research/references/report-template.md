# Research Report Template

Canonical template for research reports. Every field is mandatory unless marked optional.

## Structure

```markdown
# Research: {Question}

## TL;DR
**Conclusion**: [1-2 sentence answer to the research question]
**Confidence**: [High / Medium / Low / Disputed]
**Sources**: [N] independent, [M] single-sourced, [K] disputed

## Method
- **Search angles**: [list of 2-4 angles searched]
- **Sources fetched**: [total number of URLs retrieved and read]
- **Cross-verification**: [whether adversarial review was applied, and to which claims]

## Findings
### {Finding 1 — descriptive name}
- **Claim**: [precise, falsifiable statement]
- **Sources**: 
  - [{Title or description}]({URL}) — retrieved {YYYY-MM-DD} — [{authority type}: {1-sentence relevance}]
  - ...
- **Verification**: [Confirmed / Single-sourced / Disputed]
- **Confidence**: [High / Medium / Low]

[Repeat for each finding]

## Cross-Check Results
| Status | Count | Description |
|--------|-------|-------------|
| Confirmed (≥2 independent sources) | N | Claims verified |
| Single-sourced (1 source only) | M | Needs additional verification |
| Disputed (conflicting sources) | K | Unresolved |

## Adversarial Review
[If applied: what claim was reviewed, what the adversarial sub-agent found, and whether the claim survived]

## Disputed / Unresolved
- **Claim**: [what is disputed]
- **Source A says**: [position + URL]
- **Source B says**: [position + URL]
- **Resolution path**: [what would be needed to determine ground truth]

## Open Questions
- [Question that remains after this research]
- [Suggested next research direction]

## Source Index
| # | URL | Title/Description | Type | Retrieved | Authority |
|---|-----|-------------------|------|-----------|-----------|
| 1 | https://... | ... | Official docs | YYYY-MM-DD | High |
| 2 | https://... | ... | Peer-reviewed paper | YYYY-MM-DD | High |
| 3 | https://... | ... | Blog post | YYYY-MM-DD | Medium |
```

## Authority Classification

| Type | Weight | Examples |
|------|--------|----------|
| **Official documentation** | High | api.example.com/docs, pkg.go.dev, docs.rs |
| **Peer-reviewed paper** | High | arxiv.org, ACM, IEEE |
| **Official announcement** | High | Company blog, release notes, changelog |
| **GitHub README/source** | Medium | Primary repo documentation |
| **Technical blog (recognized author)** | Medium | Domain expert's technical write-up |
| **Community wiki / Stack Overflow** | Low-Medium | Depends on answer quality and votes |
| **GitHub issue comment** | Low | Individual report, unverified |
| **Social media post** | Low | Claim verification needed |

## Confidence Levels

| Level | Criteria |
|-------|----------|
| **High** | ≥3 independent, authoritative sources agree. Adversarial review passed. |
| **Medium** | ≥2 independent sources agree. May include 1 medium-authority source. |
| **Low** | Only 1 source, or sources are low-authority, or adversarial review found issues. |
| **Disputed** | Independent, authoritative sources disagree. Both positions reported. |
