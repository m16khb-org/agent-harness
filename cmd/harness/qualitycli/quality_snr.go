package qualitycli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// snrEvidence renders the SNR signal's human-readable evidence lines.
func snrEvidence(snr SNRResult) []string {
	return []string{
		fmt.Sprintf("signal=%d noise=%d total=%d lines", snr.SignalLines, snr.NoiseLines, snr.TotalLines),
		"Shannon-style code signal-to-noise (logic vs blank/comment/structural); higher is denser",
	}
}

const snrBaselineSchemaVersion = 1

type snrBaselineRecord struct {
	SchemaVersion int     `json:"schema_version"`
	Repository    string  `json:"repository"`
	Ratio         float64 `json:"ratio"`
}

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
func computeCodeSNR(root string) (SNRResult, error) {
	var signal, noise int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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
		s, n, err := snrCountFile(path)
		if err != nil {
			return err
		}
		signal += s
		noise += n
		return nil
	})
	if err != nil {
		return SNRResult{}, err
	}
	total := signal + noise
	ratio := 0.0
	if total > 0 {
		ratio = math.Round(float64(signal)/float64(total)*10000) / 10000
	}
	return SNRResult{SignalLines: signal, NoiseLines: noise, TotalLines: total, Ratio: ratio}, nil
}

func snrCountFile(path string) (signal, noise int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
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
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	return signal, noise, nil
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

// readSNRBaseline distinguishes an absent baseline from corrupted or
// unavailable state so --trend cannot silently suppress a regression.
func readSNRBaseline(root string) (float64, bool, error) {
	if hostDeps.StateRead == nil {
		return 0, false, fmt.Errorf("quality SNR baseline store is not configured")
	}
	repository, key, err := snrBaselineIdentity(root)
	if err != nil {
		return 0, false, err
	}
	res, err := hostDeps.StateRead(key)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !res.OK {
		return 0, false, fmt.Errorf("quality SNR baseline read returned no record")
	}
	var baseline snrBaselineRecord
	if err := json.Unmarshal([]byte(res.Record.Content), &baseline); err != nil {
		return 0, false, fmt.Errorf("parse quality SNR baseline: %w", err)
	}
	if baseline.SchemaVersion != snrBaselineSchemaVersion ||
		baseline.Repository != repository ||
		!validSNRRatio(baseline.Ratio) {
		return 0, false, fmt.Errorf("quality SNR baseline identity or ratio is invalid")
	}
	return baseline.Ratio, true, nil
}

// saveSNRBaseline persists the current ratio as the new baseline for trend
// comparison on a later run.
func saveSNRBaseline(root string, ratio float64) error {
	if hostDeps.StateWrite == nil {
		return fmt.Errorf("quality SNR baseline store is not configured")
	}
	if !validSNRRatio(ratio) {
		return fmt.Errorf("quality SNR baseline ratio must be finite and between zero and one")
	}
	repository, key, err := snrBaselineIdentity(root)
	if err != nil {
		return err
	}
	content, err := json.Marshal(snrBaselineRecord{
		SchemaVersion: snrBaselineSchemaVersion,
		Repository:    repository,
		Ratio:         ratio,
	})
	if err != nil {
		return err
	}
	_, err = hostDeps.StateWrite(key, string(content))
	return err
}

func snrBaselineIdentity(root string) (string, string, error) {
	repository, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve quality SNR repository: %w", err)
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		return "", "", fmt.Errorf("resolve quality SNR repository symlinks: %w", err)
	}
	repository = filepath.Clean(repository)
	sum := sha256.Sum256([]byte(repository))
	return repository, "quality-snr-baseline-" + hex.EncodeToString(sum[:8]), nil
}

func validSNRRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio >= 0 && ratio <= 1
}
