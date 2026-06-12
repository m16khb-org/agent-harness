// Package skillcontract pins safety- and contract-critical phrases of skills
// that have no dedicated Go test coverage (program item S3). Like the
// issueops skill contract test, these assert TEXT PRESENCE only — they do
// not prove runtime behavior; behavioral coverage belongs to measurement
// cases and domain packages.
package skillcontract
