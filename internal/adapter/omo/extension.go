package omo

import (
	"encoding/json"
	"fmt"
)

func omoLifecycleExtension(binPath string) string {
	encodedBin, _ := json.Marshal(binPath)
	return fmt.Sprintf(`const harnessBin = %s

async function injectProjectDocs(pi, subcommand, ctx) {
  try {
    const result = await pi.exec(
      harnessBin,
      ["hook", subcommand, "--repo", ctx.cwd, "--json"],
      { cwd: ctx.cwd },
    )
    if (result.code !== 0) {
      ctx.ui.notify("issueops lifecycle hook failed", "warning")
      return
    }
    const payload = JSON.parse(result.stdout)
    if (!payload.should_inject || !payload.compact) return
    pi.sendMessage(
      {
        customType: "issueops:project-docs",
        content: payload.compact,
        display: false,
      },
      { triggerTurn: false },
    )
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    ctx.ui.notify("issueops lifecycle hook failed: " + detail, "warning")
  }
}

export default function agentHarness(pi) {
  pi.on("session_start", (_event, ctx) =>
    injectProjectDocs(pi, "session-start", ctx),
  )
  pi.on("session_compact", (event, ctx) => {
    if (!event.accepted) return
    return injectProjectDocs(pi, "post-compact", ctx)
  })
}
`, encodedBin)
}

func LifecycleExtension(binPath string) string {
	return omoLifecycleExtension(binPath)
}
