package rejection

import (
	"errors"
	"fmt"
)

// Code is a stable Independent Check Contract rejection code.
type Code string

const (
	ArtifactMissing            Code = "artifact-missing"
	DigestMismatch             Code = "digest-mismatch"
	DuplicateArtifact          Code = "duplicate-artifact"
	DuplicateMember            Code = "duplicate-member"
	EvidenceCardinality        Code = "evidence-cardinality"
	InvalidJSON                Code = "invalid-json"
	InvalidStateSupport        Code = "invalid-state-support"
	InvalidUTF8                Code = "invalid-utf8"
	IsolationBoundaryViolation Code = "isolation-boundary-violation"
	JSONNumberOrNull           Code = "json-number-or-null"
	LengthMismatch             Code = "length-mismatch"
	LimitSetMismatch           Code = "limit-set-mismatch"
	ManifestCoverage           Code = "manifest-coverage"
	MissingRequiredMember      Code = "missing-required-member"
	NoncanonicalJSON           Code = "noncanonical-json"
	NoncanonicalOrder          Code = "noncanonical-order"
	ObligationMismatch         Code = "obligation-mismatch"
	RequestBindingMismatch     Code = "request-binding-mismatch"
	ResourceLimit              Code = "resource-limit"
	UnknownMember              Code = "unknown-member"
	UnknownTag                 Code = "unknown-tag"
	UnsupportedVersion         Code = "unsupported-version"
)

// Error avoids including untrusted input bytes or host paths.
type Error struct {
	Code   Code
	Offset int
	Detail string
}

func (e *Error) Error() string {
	where := ""
	if e.Offset >= 0 {
		where = fmt.Sprintf(" at byte %d", e.Offset)
	}
	if e.Detail == "" {
		return string(e.Code) + where
	}
	return fmt.Sprintf("%s%s: %s", e.Code, where, e.Detail)
}

func New(code Code, detail string) error {
	return &Error{Code: code, Offset: -1, Detail: detail}
}

func At(code Code, offset int, detail string) error {
	return &Error{Code: code, Offset: offset, Detail: detail}
}

func CodeOf(err error) (Code, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target.Code, true
	}
	return "", false
}
