// Package approval holds the types shared by the tool-approval wrapper and the
// tools that know which of their calls need a human decision.
//
// The package deliberately has no dependency on the tools package so that both
// built-in tools (including tools/custom) and front-ends can use it.
package approval

import "strings"

// Approval modes, configured with settings.approval.mode.
const (
	ModeOff       = "off"       // never ask
	ModeDangerous = "dangerous" // ask only for calls a tool flags as mutating (default)
	ModeAll       = "all"       // ask before every tool call
)

// Answers a front-end offers for an approval request.
const (
	AllowOnce    = "允许本次"
	AllowSession = "本会话允许"
	Deny         = "拒绝"
)

// Request describes one tool call that is waiting for the user's decision.
type Request struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`          // one-line description of the action
	Detail  string `json:"detail,omitempty"` // raw tool arguments, for context
}

// Classifier is implemented by tools that can tell whether a specific call is
// dangerous. The bool result is true when the call mutates state, runs code or
// otherwise deserves confirmation.
type Classifier interface {
	ApprovalRequest(arguments string) (Request, bool)
}

// Policy is the resolved approval configuration.
type Policy struct {
	Mode   string
	Allow  map[string]bool // tools that never need approval
	Always map[string]bool // tools that always need approval
}

// ModeOrDefault returns the configured mode, defaulting to ModeDangerous so a
// tool that can run commands or write files asks before doing so.
func (p Policy) ModeOrDefault() string {
	switch strings.ToLower(strings.TrimSpace(p.Mode)) {
	case ModeOff:
		return ModeOff
	case ModeAll:
		return ModeAll
	default:
		return ModeDangerous
	}
}

// NeedsApproval decides whether a call must be confirmed and builds the request
// shown to the user.
func (p Policy) NeedsApproval(name, arguments string, classifier Classifier) (Request, bool) {
	mode := p.ModeOrDefault()
	if mode == ModeOff || p.Allow[name] {
		return Request{}, false
	}

	req := Request{Tool: name}
	flagged := false
	if classifier != nil {
		if classified, dangerous := classifier.ApprovalRequest(arguments); dangerous {
			req = classified
			req.Tool = name
			flagged = true
		}
	}
	if p.Always[name] {
		flagged = true
	}

	switch mode {
	case ModeAll:
		// Ask in every case, but keep the classifier's summary when it has one.
	case ModeDangerous:
		if !flagged {
			return Request{}, false
		}
	default:
		return Request{}, false
	}

	if strings.TrimSpace(req.Summary) == "" {
		req.Summary = "调用工具 " + name
	}
	if req.Detail == "" {
		req.Detail = arguments
	}
	return req, true
}
