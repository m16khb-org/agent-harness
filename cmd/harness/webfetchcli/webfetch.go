package webfetchcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
)

type Deps struct {
	Stdout io.Writer
}

func Run(args []string) error {
	return RunWithDeps(args, Deps{Stdout: os.Stdout})
}

func RunWithDeps(args []string, deps Deps) error {
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(deps.Stdout, "Usage: agent-harness web-fetch fetch|benchmark [--json]")
		return nil
	}
	switch args[0] {
	case "fetch":
		return runFetch(args[1:], deps)
	case "benchmark":
		return runBenchmark(args[1:], deps)
	default:
		return fmt.Errorf("unknown web-fetch subcommand %q", args[0])
	}
}

func runFetch(args []string, deps Deps) error {
	fs := flag.NewFlagSet("web-fetch fetch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rawURL := fs.String("url", "", "URL to fetch")
	timeoutText := fs.String("timeout", "30s", "request timeout")
	maxChars := fs.Int("max-chars", 0, "maximum content characters")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *rawURL == "" {
		return fmt.Errorf("--url is required")
	}
	timeout, err := time.ParseDuration(*timeoutText)
	if err != nil {
		return fmt.Errorf("invalid --timeout: %w", err)
	}
	result, err := Fetch(context.Background(), webfetchcontract.Request{
		URL:      *rawURL,
		Timeout:  timeout,
		MaxChars: *maxChars,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(deps.Stdout, result)
	}
	if result.Content != "" {
		fmt.Fprintln(deps.Stdout, result.Content)
		return nil
	}
	fmt.Fprintf(deps.Stdout, "%s: %s\n", result.Category, result.StopReason)
	return nil
}

func runBenchmark(args []string, deps Deps) error {
	fs := flag.NewFlagSet("web-fetch benchmark", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fixturesPath := fs.String("fixtures", "", "fixture path or builtin")
	live := fs.Bool("live", false, "run opt-in live benchmark")
	compareBaseline := fs.String("compare-baseline", "", "optional baseline comparator executable")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fixturesPath == "" {
		return fmt.Errorf("--fixtures is required")
	}
	fixtures, err := loadFixtures(*fixturesPath)
	if err != nil {
		return err
	}
	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Fixtures:       fixtures,
		Live:           *live,
		LiveOptIn:      os.Getenv("HARNESS_WEBFETCH_LIVE") == "1",
		CompareCommand: *compareBaseline,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(deps.Stdout, result)
	}
	fmt.Fprintf(deps.Stdout, "web-fetch benchmark score %.1f fixture_count=%d ok=%v\n", result.Score, result.FixtureCount, result.OK)
	return nil
}

func printJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func loadFixtures(path string) ([]webfetchcontract.BenchmarkFixture, error) {
	if path == "builtin" {
		return DeterministicFixtures(), nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return loadFixtureFile(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	var fixtures []webfetchcontract.BenchmarkFixture
	for _, name := range names {
		next, err := loadFixtureFile(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, next...)
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("fixture directory %q contains no JSON fixtures", path)
	}
	return fixtures, nil
}

func loadFixtureFile(path string) ([]webfetchcontract.BenchmarkFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Fixtures []webfetchcontract.BenchmarkFixture `json:"fixtures"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Fixtures != nil {
		return wrapper.Fixtures, nil
	}
	var one webfetchcontract.BenchmarkFixture
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, fmt.Errorf("parse fixtures %s: %w", path, err)
	}
	if one.ID == "" {
		return nil, fmt.Errorf("fixture file %s missing id", path)
	}
	return []webfetchcontract.BenchmarkFixture{one}, nil
}
