# Kordoc Capability Contract

Re-probe the current host before every analysis. This file records the verified
2026-07-23 local snapshot and the decision rules; it is not permission to assume
future versions behave the same way.

## Expected Tool Surface

| Tool | Verified schema |
|---|---|
| `detect_format` | `file_path: string` |
| `parse_document` | `file_path: string`, optional `ocr: boolean` |
| `parse_pages` | `file_path: string`, `pages: string` |

Use the host's tool-discovery mechanism to obtain current names and schemas.
Never invent a namespace or reuse a stale schema.

## 2026-07-23 Read-Only Probe

Fixture: temporary two-page Korean mixed PDF containing native body text and
embedded UI screenshots.

| Check | Result | Direct evidence | Limit |
|---|---|---|---|
| MCP connection | working | `parse_document` returned Markdown | Local streamable HTTP proxy only |
| Format detection | working for PDF | magic-byte result `pdf` | `detect_format` rejected `.png` |
| Native parse | working | four native facts and two page-level image warnings | Embedded screenshot text omitted |
| Source `ocr=true` | working but insufficient | output exactly matched native parse | Normal text pages skipped embedded-image OCR |
| Direct image input | rejected | `.png` failed extension validation | Contradicts the tool description/schema |
| Image-only PDF OCR | working | two pages received `OCR_APPLIED`; Korean text extracted | Currency/spacing/email errors remained |
| Repeated OCR | reproducible | two identical image-only-PDF outputs | Re-probe per version/input |
| Backend identity | verified for the active local process | configured process used `kordoc@4.2.7`; installed model specs and status-only SHA checks identified mobile det and Korean rec | Does not prove a different remote server's engine |

The configured MCP transport was
`http://127.0.0.1:7351/servers/kordoc/mcp`. The active proxy command identified
`kordoc@4.2.7`. Installed source named:

- detection: `PP-OCRv5 mobile det`
- recognition: `PP-OCRv5 korean rec`
- dictionary: `PP-OCRv5 korean dict`

The installed CLI's `check-ocr-models --status-only` reported all three files
present and verified. This is direct evidence for this local process, not a
general promise about every Kordoc deployment.

## Probe Procedure

1. Inspect the configured Kordoc transport without exposing secret values.
2. Discover the actual tools and input schemas.
3. Call `detect_format` on the source.
4. Call `parse_document(ocr=false)` and preserve warnings.
5. Call `parse_document(ocr=true)` once on the original.
6. Probe one temporary PNG/JPG. If rejected, record the failure and stop retrying.
7. Render every page and wrap the images in a text-layer-free PDF.
8. Call `parse_document(ocr=true)` twice on that exact PDF and compare output.
9. Inspect current server metadata, active installed source/configuration, logs,
   or model status for backend identity.

Do not use `npx -y`, package-manager install commands, or a model-download
command when `ALLOW_INSTALL=false`. If an already installed executable exposes
a status-only command, it may be used read-only.

## Status Rules

- OCR output with real Korean text: functionality `working`.
- Tool description only: backend `claimed-but-unverified`.
- Active process plus installed source/model checksum status: backend `verified`
  for that process and version.
- No direct evidence: never upgrade the backend label beyond
  `claimed-but-unverified`.
- Successful `ocr=true` on a mixed PDF does not prove embedded screenshots were
  OCRed.
- Successful image-only-PDF OCR does not by itself identify the OCR engine.

## Fallback

If direct image input fails:

1. preserve the error once;
2. build an image-only PDF;
3. verify its page count and empty text layer;
4. OCR that PDF;
5. keep `full-page-ocr` and `region-ocr` outputs separate.
