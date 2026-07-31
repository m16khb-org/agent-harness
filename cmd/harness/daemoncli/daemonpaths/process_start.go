package daemonpaths

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func processFieldWithCLocale(pid int, field string) ([]byte, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", field)
	env := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "LANG=") ||
			strings.HasPrefix(value, "LC_ALL=") ||
			strings.HasPrefix(value, "LC_TIME=") {
			continue
		}
		env = append(env, value)
	}
	cmd.Env = append(env, "LANG=C", "LC_ALL=C", "LC_TIME=C")
	return cmd.Output()
}

func canonicalProcessStartTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("process start time is empty")
	}
	if strings.HasPrefix(value, "linux:") {
		return value, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339), nil
	}
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}

	var year, month, day, hour, minute, second int
	var weekday string
	if count, err := fmt.Sscanf(
		value,
		"%d년 %d월 %d일 %s %d시 %d분 %d초",
		&year,
		&month,
		&day,
		&weekday,
		&hour,
		&minute,
		&second,
	); err == nil && count == 7 {
		parsed := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
		if parsed.Year() == year &&
			int(parsed.Month()) == month &&
			parsed.Day() == day &&
			parsed.Hour() == hour &&
			parsed.Minute() == minute &&
			parsed.Second() == second {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("unsupported process start time %q", value)
}

// ProcessStartTimeEqual은 로케일이 달랐던 구버전 daemon receipt와 현재의
// 로케일 독립 receipt가 같은 OS 프로세스를 가리키는지 비교한다.
func ProcessStartTimeEqual(recorded, observed string) bool {
	if recorded == observed {
		return true
	}
	recordedCanonical, err := canonicalProcessStartTime(recorded)
	if err != nil {
		return false
	}
	observedCanonical, err := canonicalProcessStartTime(observed)
	return err == nil && recordedCanonical == observedCanonical
}
