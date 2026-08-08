package qualitycli

import (
	"bufio"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// snrEvidence renders the SNR signal's human-readable evidence lines.
func snrEvidence(snr SNRResult) []string {
	return []string{
		fmt.Sprintf("signal=%d noise=%d total=%d lines", snr.SignalLines, snr.NoiseLines, snr.TotalLines),
		"Shannon-style code signal-to-noise (logic vs blank/comment/structural); higher is denser",
	}
}

// snrBaselineStateKey is the harness-state key under which the most recent
// code-SNR ratio is persisted so a later run can report a trend delta.
const snrBaselineStateKey = "quality-snr-baseline"

// SNRResult is a deterministic Shannon-style signal-to-noise measure over the
// repository's production Go source: signal lines (logic) versus noise lines
// (blank, comment-only, or structural-only such as a lone brace). It is a
// quantitative code-quality proxy — higher Ratio means less channel overhead.
// It does not judge whether the logic is correct, only its density.
type SNRResult struct {
	SignalLines int     `json:"signal_lines"`
	NoiseLines  int     `json:"noise_lines"`
	TotalLines  int     `json:"total_lines"`
	Ratio       float64 `json:"ratio"`
}

// computeCodeSNR walks root for production (non-test) Go files and computes the
// signal-to-noise ratio. It is deterministic for a given file tree.
func computeCodeSNR(root string) SNRResult {
	var signal, noise int
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		s, n := snrCountFile(path)
		signal += s
		noise += n
		return nil
	})
	total := signal + noise
	ratio := 0.0
	if total > 0 {
		ratio = math.Round(float64(signal)/float64(total)*10000) / 10000
	}
	return SNRResult{SignalLines: signal, NoiseLines: noise, TotalLines: total, Ratio: ratio}
}

func snrCountFile(path string) (signal, noise int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inBlockComment := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case inBlockComment:
			noise++
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
		case line == "":
			noise++
		case strings.HasPrefix(line, "//"):
			noise++
		case strings.HasPrefix(line, "/*"):
			noise++
			if !strings.Contains(line, "*/") {
				inBlockComment = true
			}
		case snrStructuralOnly(line):
			noise++
		default:
			signal++
		}
	}
	return signal, noise
}

// snrStructuralOnly reports whether a line carries only block-structure runes
// (braces, parens, commas) and therefore no logic signal.
func snrStructuralOnly(line string) bool {
	for _, r := range line {
		switch r {
		case '{', '}', '(', ')', ',':
		default:
			return false
		}
	}
	return line != ""
}

// readSNRBaseline returns the persisted baseline ratio, or false when none is
// stored or it cannot be parsed.
func readSNRBaseline() (float64, bool) {
	if hostDeps.StateRead == nil {
		return 0, false
	}
	res, err := hostDeps.StateRead(snrBaselineStateKey)
	if err != nil || !res.OK {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(res.Record.Content), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// saveSNRBaseline persists the current ratio as the new baseline for trend
// comparison on a later run.
func saveSNRBaseline(ratio float64) error {
	if hostDeps.StateWrite == nil {
		return fmt.Errorf("quality SNR baseline store is not configured")
	}
	_, err := hostDeps.StateWrite(snrBaselineStateKey, strconv.FormatFloat(ratio, 'f', 4, 64))
	return err
}
