// Package channelcli는 세션 간 메시지 채널 CLI의 flag 해석과 출력을 소유한다.
// 저장소 조립은 composition root가 하고(Dependencies), 여기서는 메시지 형식을
// 모른다.
package channelcli

import (
	"encoding/json"
	"flag"
	"fmt"
	channelcontract "issueops/internal/contract/channel"
	"os"
)

// Dependencies는 channel CLI가 필요한 연산을 함수로 받는다.
type Dependencies struct {
	Send func(channelcontract.SendRequest) (channelcontract.SendResult, error)
	Recv func(channelcontract.RecvRequest) (channelcontract.RecvResult, error)
}

// TimedOutError는 recv --wait가 시간 안에 메시지를 못 본 종료 오류이다(exit 1).
type TimedOutError struct{}

func (TimedOutError) Error() string { return "wait timed out with no new messages" }

func Run(deps Dependencies, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		channelUsage()
		return nil
	}
	switch args[0] {
	case "send":
		return runSend(deps, args[1:])
	case "recv":
		return runRecv(deps, args[1:])
	default:
		channelUsage()
		return fmt.Errorf("unknown channel subcommand %q", args[0])
	}
}

func channelUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  issueops channel send --channel NAME --from SESSION --message TEXT [--json]
  issueops channel recv --channel NAME [--since MSG_ID] [--wait] [--timeout-seconds N] [--limit N] [--json]

recv returns messages after --since (exclusive). --wait blocks until a new
message arrives or the timeout (default 300s). Exit codes: 0 messages returned,
1 wait timed out with no messages, 2 usage error.
`)
}

func runSend(deps Dependencies, args []string) error {
	fs := flag.NewFlagSet("channel send", flag.ContinueOnError)
	channelName := fs.String("channel", "", "channel name (shared mailbox key)")
	from := fs.String("from", "", "sending session identity, e.g. server|front|codex:abc")
	message := fs.String("message", "", "message body")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := deps.Send(channelcontract.SendRequest{Channel: *channelName, From: *from, Body: *message})
	if *jsonOut {
		return printJSON(result)
	}
	if err != nil {
		return err
	}
	fmt.Printf("sent %s to %s (from %s)\n", result.Message.ID, result.Channel, result.Message.From)
	return nil
}

func runRecv(deps Dependencies, args []string) error {
	fs := flag.NewFlagSet("channel recv", flag.ContinueOnError)
	channelName := fs.String("channel", "", "channel name (shared mailbox key)")
	since := fs.String("since", "", "return messages after this message id (exclusive)")
	wait := fs.Bool("wait", false, "block until a new message arrives or timeout")
	timeout := fs.Int("timeout-seconds", channelcontract.DefaultWaitTimeoutSeconds, "wait timeout in seconds (with --wait)")
	limit := fs.Int("limit", 0, "maximum messages to return (0 = no limit)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := deps.Recv(channelcontract.RecvRequest{
		Channel:        *channelName,
		SinceID:        *since,
		Wait:           *wait,
		TimeoutSeconds: *timeout,
		Limit:          *limit,
	})
	if *jsonOut {
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else if err == nil {
		printRecvText(result)
	}
	if err != nil {
		return err
	}
	if result.TimedOut {
		return TimedOutError{}
	}
	return nil
}

func printRecvText(result channelcontract.RecvResult) {
	for _, msg := range result.Messages {
		fmt.Printf("%s [%s] %s: %s\n", msg.CreatedAt, msg.ID, msg.From, msg.Body)
	}
	if result.TimedOut {
		fmt.Println("(wait timed out, no new messages)")
	} else if len(result.Messages) == 0 {
		fmt.Println("(no messages)")
	}
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
