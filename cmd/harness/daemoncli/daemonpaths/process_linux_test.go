//go:build linux

package daemonpaths

import (
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

func TestLinuxProcessStartTimePreservesClockTickPrecision(t *testing.T) {
	systemStat := []byte("cpu 1 2 3 4\nbtime 1700000000\n")
	auxv := linuxTestAuxv([2]uint64{17, 100}, [2]uint64{0, 0})

	tests := []struct {
		name       string
		startTicks string
		want       string
	}{
		{name: "half second", startTicks: "250", want: "2023-11-14T22:13:22.5Z"},
		{name: "adjacent tick", startTicks: "251", want: "2023-11-14T22:13:22.51Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := linuxProcessStartTime(linuxTestProcessStat(tt.startTicks), systemStat, auxv)
			if err != nil || got != tt.want {
				t.Fatalf("linux process start time mismatch: got=%q want=%q err=%v", got, tt.want, err)
			}
		})
	}
}

func TestLinuxProcessStartTimeRejectsInvalidKernelData(t *testing.T) {
	validStat := linuxTestProcessStat("250")
	validSystemStat := []byte("btime 1700000000\n")
	validAuxv := linuxTestAuxv([2]uint64{17, 100}, [2]uint64{0, 0})

	tests := []struct {
		name       string
		stat       []byte
		systemStat []byte
		auxv       []byte
	}{
		{name: "missing start ticks", stat: linuxTestProcessStat(""), systemStat: validSystemStat, auxv: validAuxv},
		{name: "missing boot time", stat: validStat, systemStat: []byte("cpu 1 2 3 4\n"), auxv: validAuxv},
		{name: "missing clock ticks", stat: validStat, systemStat: validSystemStat, auxv: linuxTestAuxv([2]uint64{0, 0})},
		{name: "zero clock ticks", stat: validStat, systemStat: validSystemStat, auxv: linuxTestAuxv([2]uint64{17, 0}, [2]uint64{0, 0})},
		{name: "unrepresentable clock ticks", stat: validStat, systemStat: validSystemStat, auxv: linuxTestAuxv([2]uint64{17, 1_000_000_001}, [2]uint64{0, 0})},
		{name: "truncated auxv", stat: validStat, systemStat: validSystemStat, auxv: validAuxv[:len(validAuxv)-1]},
		{name: "start seconds overflow int64", stat: linuxTestProcessStat("18446744073709551615"), systemStat: validSystemStat, auxv: linuxTestAuxv([2]uint64{17, 1}, [2]uint64{0, 0})},
		{name: "start epoch addition overflow", stat: validStat, systemStat: []byte("btime 9223372036854775807\n"), auxv: validAuxv},
		{name: "timestamp outside RFC3339 year range", stat: linuxTestProcessStat("0"), systemStat: []byte("btime 253402300800\n"), auxv: validAuxv},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := linuxProcessStartTime(tt.stat, tt.systemStat, tt.auxv); err == nil {
				t.Fatalf("expected invalid kernel data to fail closed, got=%q", got)
			}
		})
	}
}

func linuxTestProcessStat(startTicks string) []byte {
	fields := make([]string, 20)
	fields[0] = "S"
	for i := 1; i < 19; i++ {
		fields[i] = "0"
	}
	fields[19] = startTicks
	return []byte("123 (agent harness) " + strings.Join(fields, " "))
}

func linuxTestAuxv(entries ...[2]uint64) []byte {
	wordSize := strconv.IntSize / 8
	data := make([]byte, len(entries)*wordSize*2)
	for i, entry := range entries {
		offset := i * wordSize * 2
		if wordSize == 8 {
			binary.NativeEndian.PutUint64(data[offset:], entry[0])
			binary.NativeEndian.PutUint64(data[offset+wordSize:], entry[1])
			continue
		}
		binary.NativeEndian.PutUint32(data[offset:], uint32(entry[0]))
		binary.NativeEndian.PutUint32(data[offset+wordSize:], uint32(entry[1]))
	}
	return data
}
