package core

import (
	"regexp"
)

var ambiguousTestNameRe = regexp.MustCompile(`(?i)\b(Test(Works|Basic|Test[0-9]*)|it\s*\(\s*["'](works|basic|test)[^"']*["']|test\s*\(\s*["'](works|basic|test)[^"']*["'])`)

var sleepInTestRe = regexp.MustCompile(`(?i)\b(time\.Sleep|Thread\.sleep|sleep\s*\(|setTimeout\s*\()`)

var externalURLRe = regexp.MustCompile(`https?://[^\s"')]+`)

var snapshotAssertionRe = regexp.MustCompile(`(?i)(toMatchSnapshot|assert.*golden|golden mismatch)`)

var newSymbolRe = regexp.MustCompile(`^\s*(?:func\s+|function\s+|def\s+|class\s+|type\s+|interface\s+|const\s+|let\s+|var\s+)([A-Za-z_][A-Za-z0-9_]*)`)

// Immutable-prefix determinism guard (Reasonix-derived). A file that opts in
// with the immutablePrefixMarker comment declares that it builds context an
// agent reuses as a stable cache prefix; introducing wall-clock/random/nonce
// values there breaks byte-determinism. A line may carry volatileOKMarker to
// declare the value intentionally lives in a volatile region.

const immutablePrefixMarker = "harness:immutable-prefix"

const volatileOKMarker = "volatile-ok"

var contextNonDeterminismRe = regexp.MustCompile(`(time\.Now\(|time\.Since\(|rand\.(?:Int|Intn|Float32|Float64|Read|Perm|Shuffle)|uuid\.New)`)
