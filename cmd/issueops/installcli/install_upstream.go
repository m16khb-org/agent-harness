package installcli

import (
	"context"
	"time"

	upstreamcontract "issueops/internal/contract/upstream"
	"issueops/internal/port"
)

// upstreamSyncTimeout bounds how long install waits on upstream provisioning.
// Provisioning talks to a third-party CLI and to git remotes, so it is capped
// rather than allowed to stall the install it runs inside.
const upstreamSyncTimeout = 5 * time.Minute

// appendUpstreamMessages provisions declared upstream plugins and skills and
// records the outcome as install messages. It never changes install success:
// the issueops install path must stay independent of third-party CLIs and
// network reachability, so upstream problems are reported, not fatal.
func appendUpstreamMessages(result *port.NativeInstallResult, root string, dryRun bool) {
	if deps.SyncUpstream == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), upstreamSyncTimeout)
	defer cancel()

	report, err := deps.SyncUpstream(ctx, root, dryRun)
	if err != nil {
		result.Messages = append(result.Messages, "upstream sync did not run: "+err.Error())
		return
	}
	for _, item := range report.Items {
		result.Messages = append(result.Messages, formatUpstreamItem(item))
	}
}

func formatUpstreamItem(item upstreamcontract.ItemResult) string {
	message := "upstream " + item.Kind + " " + item.Name + ": " + item.Status
	detail := item.Error
	if detail == "" {
		detail = item.Reason
	}
	if detail != "" {
		message += " (" + detail + ")"
	}
	return message
}
