const string = { type: "string" }
const integer = { type: "integer" }
const nullableInteger = { type: ["integer", "null"] }
const nullableString = { type: ["string", "null"] }
const stringArray = { type: "array", items: string }

const candidate = {
  type: "object",
  additionalProperties: false,
  properties: {
    path: string,
    new_line: integer,
    end_line: nullableInteger,
    severity: { type: "string", enum: ["critical", "high", "medium", "low"] },
    category: {
      type: "string",
      enum: ["bug", "security", "performance", "business-logic", "data", "api-contract", "test", "rule", "scope"],
    },
    title: string,
    what: string,
    why: string,
    how: string,
    evidence: stringArray,
    upstream: string,
    downstream: string,
    suggestion: nullableString,
    rule: nullableString,
    confidence: integer,
    newly_reachable: { type: "boolean" },
    lens: string,
  },
  required: ["path", "new_line", "severity", "category", "title", "what", "why", "how", "evidence", "confidence", "lens"],
}

const finder = {
  type: "object",
  additionalProperties: false,
  properties: {
    lenses: stringArray,
    reviewed_files: stringArray,
    inspected: stringArray,
    candidates: { type: "array", items: candidate },
    verified_ok: {
      type: "array",
      items: {
        type: "object",
        additionalProperties: false,
        properties: {
          concern: string,
          why_ok: string,
          loc: string,
          thread: nullableString,
        },
        required: ["concern", "why_ok", "loc"],
      },
    },
  },
  required: ["lenses", "reviewed_files", "inspected", "candidates", "verified_ok"],
}

const verdict = {
  type: "object",
  additionalProperties: false,
  properties: {
    skeptic: { type: "string", enum: ["tracer", "reproducer"] },
    refuted: { type: "boolean" },
    confidence: integer,
    reason: string,
    evidence: stringArray,
    severity_adjust: { type: "string", enum: ["keep", "lower", "raise"] },
    corrected_line: nullableInteger,
    corrected_suggestion: nullableString,
  },
  required: ["skeptic", "refuted", "confidence", "reason", "evidence", "severity_adjust"],
}

const registerOutputTool = (pi, name, label, description, parameters, onSubmit) => {
  pi.registerTool({
    name,
    label,
    description,
    parameters,
    constrainedSampling: { type: "json_schema", strict: "require" },
    async execute(_toolCallId, params) {
      onSubmit()
      return {
        content: [{ type: "text", text: "Structured result accepted. End the turn now." }],
        details: { payload: params },
      }
    },
  })
}

export default function parnasStructuredOutput(pi) {
  pi.registerFlag("parnas-max-turns", {
    type: "string",
    description: "Reserve the final Parnas agent turn for structured output",
  })
  let turns = 0
  let submitted = false
  pi.on("turn_end", () => {
    turns += 1
    const maxTurns = Number.parseInt(String(pi.getFlag("parnas-max-turns") || ""), 10)
    if (submitted || !Number.isInteger(maxTurns) || maxTurns < 1 || turns < maxTurns - 1) return
    const outputTool = pi.getActiveTools().find((name) => name.startsWith("submit_parnas_"))
    if (outputTool) pi.setActiveTools([outputTool])
  })

  registerOutputTool(
    pi,
    "submit_parnas_finder",
    "Submit Parnas finder result",
    "Required final action. Submit the complete finder result once; do not print it as assistant text.",
    finder,
    () => { submitted = true },
  )
  registerOutputTool(
    pi,
    "submit_parnas_verdict",
    "Submit Parnas verdict",
    "Required final action. Submit the complete tracer or reproducer verdict once; do not print it as assistant text.",
    verdict,
    () => { submitted = true },
  )
}
