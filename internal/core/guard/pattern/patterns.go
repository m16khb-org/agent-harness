package pattern

import "regexp"

var AmbiguousTestName = regexp.MustCompile(`(?i)\b(Test(Works|Basic|Test[0-9]*)|it\s*\(\s*["'](works|basic|test)[^"']*["']|test\s*\(\s*["'](works|basic|test)[^"']*["'])`)

var SleepInTest = regexp.MustCompile(`(?i)\b(time\.Sleep|Thread\.sleep|sleep\s*\(|setTimeout\s*\()`)

var ExternalURL = regexp.MustCompile(`https?://[^\s"')]+`)

var SnapshotAssertion = regexp.MustCompile(`(?i)(toMatchSnapshot|assert.*golden|golden mismatch)`)

var NewSymbol = regexp.MustCompile(`^\s*(?:func\s+|function\s+|def\s+|class\s+|type\s+|interface\s+|const\s+|let\s+|var\s+)([A-Za-z_][A-Za-z0-9_]*)`)

// Immutable-prefix determinism guard. A file that opts in
// with ImmutablePrefixMarker declares that it builds context an agent reuses as
// a stable cache prefix; introducing wall-clock/random/nonce values there
// breaks byte-determinism. A line may carry VolatileOKMarker to declare the
// value intentionally lives in a volatile region.
const ImmutablePrefixMarker = "harness:immutable-prefix"

const VolatileOKMarker = "volatile-ok"

var ContextNonDeterminism = regexp.MustCompile(`(time\.Now\(|time\.Since\(|rand\.(?:Int|Intn|Float32|Float64|Read|Perm|Shuffle)|uuid\.New)`)
