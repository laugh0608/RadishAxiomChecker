package checkresult

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	invocationFailureCode    = "checker-process-failure"
	invocationFailureFormat  = "axiom-checker-invocation-failure"
	invocationFailureVersion = "0.1"
	invocationNoResult       = "not-produced"
)

// InvocationFailure is deliberately not a four-state checker result. It binds
// a valid request to the fact that the outer invocation produced no result.
type InvocationFailure struct {
	Code                  string
	RequestContentDigest  protocol.Digest
	RequestDocumentDigest protocol.Digest
	ContentDigest         protocol.Digest
	raw                   []byte
}

func (failure InvocationFailure) Bytes() []byte {
	return append([]byte(nil), failure.raw...)
}

// NewInvocationFailure forms the locked outer-process failure record only when
// canonical request bytes and their document identity are available. Missing
// or malformed request bytes cannot be upgraded into a four-state result or a
// falsely bound canonical failure record.
func NewInvocationFailure(requestRaw []byte, limits strictjson.Limits) (InvocationFailure, error) {
	request, err := protocol.ParseRequest(requestRaw, limits)
	if err != nil {
		return InvocationFailure{}, fmt.Errorf("invocation failure cannot bind a noncanonical request: %w", err)
	}
	content := protocol.Digest(sha256.Sum256(requestRaw))
	failure := InvocationFailure{
		Code:                  invocationFailureCode,
		RequestContentDigest:  content,
		RequestDocumentDigest: request.DomainDigest,
	}
	raw := encodeInvocationFailure(failure)
	parsed, err := ParseInvocationFailure(raw, limits)
	if err != nil {
		return InvocationFailure{}, fmt.Errorf("encoded invocation failure failed self-check: %w", err)
	}
	return parsed, nil
}

// ParseInvocationFailure validates the canonical failure envelope. It does not
// claim the process actually failed; that observation remains the launcher's
// responsibility.
func ParseInvocationFailure(raw []byte, limits strictjson.Limits) (InvocationFailure, error) {
	value, err := strictjson.ParseCanonical(raw, limits)
	if err != nil {
		return InvocationFailure{}, err
	}
	root, err := companionObject(value, []string{"code", "format", "format_version", "request", "result"})
	if err != nil {
		return InvocationFailure{}, err
	}
	code, err := companionText(root["code"])
	if err != nil {
		return InvocationFailure{}, err
	}
	if code != invocationFailureCode {
		return InvocationFailure{}, rejection.New(rejection.UnknownTag, "unknown invocation failure code")
	}
	format, err := companionText(root["format"])
	if err != nil {
		return InvocationFailure{}, err
	}
	if format != invocationFailureFormat {
		return InvocationFailure{}, rejection.New(rejection.UnknownTag, "unknown invocation failure format")
	}
	version, err := companionText(root["format_version"])
	if err != nil {
		return InvocationFailure{}, err
	}
	if version != invocationFailureVersion {
		return InvocationFailure{}, rejection.New(rejection.UnsupportedVersion, "unsupported invocation failure version")
	}
	requestFields, err := companionObject(root["request"], []string{"content_digest", "document_digest"})
	if err != nil {
		return InvocationFailure{}, err
	}
	content, err := companionDigest(requestFields["content_digest"])
	if err != nil {
		return InvocationFailure{}, err
	}
	document, err := companionDigest(requestFields["document_digest"])
	if err != nil {
		return InvocationFailure{}, err
	}
	result, err := companionText(root["result"])
	if err != nil {
		return InvocationFailure{}, err
	}
	if result != invocationNoResult {
		return InvocationFailure{}, rejection.New(rejection.ResultAggregation, "invocation failure must not contain a checker result")
	}
	return InvocationFailure{
		Code:                  code,
		RequestContentDigest:  content,
		RequestDocumentDigest: document,
		ContentDigest:         protocol.Digest(sha256.Sum256(raw)),
		raw:                   append([]byte(nil), raw...),
	}, nil
}

// VerifyRequest recomputes both raw and request-domain identities from the
// actual bytes associated with the launcher observation.
func (failure InvocationFailure) VerifyRequest(requestRaw []byte, limits strictjson.Limits) error {
	request, err := protocol.ParseRequest(requestRaw, limits)
	if err != nil {
		return err
	}
	content := protocol.Digest(sha256.Sum256(requestRaw))
	if content != failure.RequestContentDigest || request.DomainDigest != failure.RequestDocumentDigest {
		return rejection.New(rejection.RequestBindingMismatch, "invocation failure request identity does not match actual bytes")
	}
	return nil
}

func encodeInvocationFailure(failure InvocationFailure) []byte {
	var output strings.Builder
	output.WriteString(`{"code":"checker-process-failure","format":"axiom-checker-invocation-failure","format_version":"0.1","request":{"content_digest":`)
	writeCompanionString(&output, failure.RequestContentDigest.String())
	output.WriteString(`,"document_digest":`)
	writeCompanionString(&output, failure.RequestDocumentDigest.String())
	output.WriteString(`},"result":"not-produced"}`)
	return []byte(output.String())
}
