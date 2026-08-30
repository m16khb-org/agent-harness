package installutil

import (
	"errors"
	"strings"

	"agent-harness/internal/port"
)

// JoinErrors collapses a slice of errors into a single semicolon-joined error,
// skipping nils, and returns nil when no non-nil error is present. Both host
// installers share this so error fan-in stays identical across hosts.
func JoinErrors(errs []error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

// Plan accumulates the files, links, messages, and errors produced by a host
// installer's steps so each step stays a single call instead of repeating the
// append-file / append-error boilerplate. Finish folds the accumulated errors
// into the result via JoinErrors so the OK flag and error fan-in are computed
// identically for every host.
type Plan struct {
	result port.HostInstallResult
	errs   []error
}

var _ port.InstallPlan = (*Plan)(nil)

// NewPlan starts an install plan for one host. The result begins OK and carries
// the dry-run flag through to Finish.
func NewPlan(host string, dryRun bool) *Plan {
	return &Plan{result: port.HostInstallResult{Host: host, OK: true, DryRun: dryRun}}
}

// Err records a non-nil error; nil errors are ignored.
func (p *Plan) Err(err error) {
	if err != nil {
		p.errs = append(p.errs, err)
	}
}

// Errs records every non-nil error in errs.
func (p *Plan) Errs(errs []error) {
	for _, err := range errs {
		p.Err(err)
	}
}

// File records one planned file together with its (possibly nil) write error.
func (p *Plan) File(file port.InstallFile, err error) {
	p.result.Files = append(p.result.Files, file)
	p.Err(err)
}

// Files records several already-planned files.
func (p *Plan) Files(files []port.InstallFile) {
	p.result.Files = append(p.result.Files, files...)
}

// Link records one planned symlink together with its (possibly nil) error.
func (p *Plan) Link(link port.InstallLink, err error) {
	p.result.Links = append(p.result.Links, link)
	p.Err(err)
}

// Links records several already-planned links.
func (p *Plan) Links(links []port.InstallLink) {
	p.result.Links = append(p.result.Links, links...)
}

// Message records one human-readable message in order.
func (p *Plan) Message(msg string) {
	p.result.Messages = append(p.result.Messages, msg)
}

// Messages records several messages in order.
func (p *Plan) Messages(msgs []string) {
	p.result.Messages = append(p.result.Messages, msgs...)
}

// Finish sets OK from the accumulated errors and returns the result plus the
// joined error (nil when every step succeeded).
func (p *Plan) Finish() (port.HostInstallResult, error) {
	if len(p.errs) > 0 {
		p.result.OK = false
		return p.result, JoinErrors(p.errs)
	}
	return p.result, nil
}
