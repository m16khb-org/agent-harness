package systemresource

import (
	"testing"
	"time"
)

func TestParseDarwinAvailablePercentUsesPercentValue(t *testing.T) {
	available, err := parseDarwinAvailablePercent("Pages free: 4\nSystem-wide memory free percentage: 42%")
	if err != nil {
		t.Fatalf("parseDarwinAvailablePercent() error = %v", err)
	}
	if available != 42 {
		t.Fatalf("parseDarwinAvailablePercent() = %v, want 42", available)
	}
}

func TestParseDarwinVMStatRejectsMissingSwapCounter(t *testing.T) {
	_, _, _, err := parseDarwinVMStat("Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages swapped in: 1.\n")
	if err == nil {
		t.Fatal("parseDarwinVMStat() error = nil, want missing field error")
	}
}

func TestParseDarwinVMStatAcceptsCurrentSwapCounterNames(t *testing.T) {
	pageSize, swapIn, swapOut, err := parseDarwinVMStat("Mach Virtual Memory Statistics: (page size of 16384 bytes)\nSwapins: 954456.\nSwapouts: 1871143.\n")
	if err != nil || pageSize != 16384 || swapIn != 954456 || swapOut != 1871143 {
		t.Fatalf("parseDarwinVMStat() = %d, %d, %d, %v", pageSize, swapIn, swapOut, err)
	}
}

func TestParsePlatformFixtures(t *testing.T) {
	load, err := parseLinuxLoad(" 1.25 0.50 0.25 1/100 200\n")
	if err != nil || load != 1.25 {
		t.Fatalf("parseLinuxLoad() = %v, %v", load, err)
	}
	total, available, err := parseLinuxMemory("MemTotal:       33554432 kB\nMemAvailable:   16777216 kB\n")
	if err != nil || total != 32<<30 || available != 16<<30 {
		t.Fatalf("parseLinuxMemory() = %d, %d, %v", total, available, err)
	}
	in, out, err := parseLinuxSwap("pswpin 12\npswpout 34\n")
	if err != nil || in != 12 || out != 34 {
		t.Fatalf("parseLinuxSwap() = %d, %d, %v", in, out, err)
	}
	pageSize, darwinIn, darwinOut, err := parseDarwinVMStat("Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages swapped in: 12.\nPages swapped out: 34.\n")
	if err != nil || pageSize != 16384 || darwinIn != 12 || darwinOut != 34 {
		t.Fatalf("parseDarwinVMStat() = %d, %d, %d, %v", pageSize, darwinIn, darwinOut, err)
	}
}

func TestSwapRateSaturatesOnOverflow(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	collector := NewCollector()
	collector.now = func() time.Time { return now }
	if got := collector.swapRate(0, 0, 2); got != 0 {
		t.Fatalf("first swap rate = %d, want 0", got)
	}
	now = now.Add(time.Second)
	if got := collector.swapRate(^uint64(0), 0, 2); got != ^uint64(0) {
		t.Fatalf("overflow swap rate = %d, want max uint64", got)
	}
}
