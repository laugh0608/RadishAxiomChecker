package checkresult

import (
	"fmt"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/axiomevidence"
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/resourcebudget"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

type InvocationKind string

const (
	InvocationResult InvocationKind = "result"
	InvocationFailed InvocationKind = "failure"
)

// Invocation is a closed in-process union. A canonical four-state companion
// and an outer not-produced failure can never coexist.
type Invocation struct {
	Kind       InvocationKind
	Companion  CompanionDocument
	Evaluation Evaluation
	Failure    InvocationFailure
	Resources  resourcebudget.Snapshot
}

// InvokeBundle performs one checker invocation with one cumulative resource
// ledger. It does not observe launcher kill, crash, or output truncation; those
// remain outer facts represented only by RecordInvocationFailure.
func InvokeBundle(
	root string,
	boundary IdentityBoundary,
	runtimeIdentity RuntimeIdentity,
	now func() time.Time,
) (Invocation, error) {
	if err := validateSourceBoundary(boundary); err != nil {
		return Invocation{}, fmt.Errorf("checker identity is unavailable for invocation: %w", err)
	}
	ledger := resourcebudget.New(now)
	verified, err := bundle.InspectInvocation(root, ledger)
	if err != nil {
		return Invocation{}, err
	}

	evidenceArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-evidence", "0.1")
	if !ok {
		return Invocation{}, rejection.New(rejection.EvidenceCardinality, "invocation has no unique Axiom Evidence v0.1 artifact")
	}
	evidenceBytes, err := verified.ReadArtifact(evidenceArtifact.ContentDigest)
	if err != nil {
		return Invocation{}, err
	}
	document, err := axiomevidence.ParseStructure(evidenceBytes, invocationJSONLimits(verified.Request, ledger))
	if err != nil {
		return Invocation{}, err
	}
	invocationBoundary := boundary
	invocationBoundary.Evidence = DocumentIdentity{
		ContentDigest: document.ContentDigest, DomainDigest: document.DomainDigest, DomainAvailable: true,
	}
	invocationBoundary.Request = DocumentIdentity{
		ContentDigest: verified.RequestDigest, DomainDigest: verified.Request.DomainDigest, DomainAvailable: true,
	}

	if finding := verified.IdentityFinding(); finding != nil {
		// A resource exhaustion accumulated during identity formation cannot
		// hide an already observed deterministic contradiction.
		_ = ledger.Activate()
		evaluation, err := earlyIdentityEvaluation(
			verified, invocationBoundary, document, CheckRejected, finding,
		)
		if err != nil {
			return Invocation{}, err
		}
		return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
	}

	irArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-ir", "0.1")
	if !ok {
		return Invocation{}, rejection.New(rejection.ManifestCoverage, "invocation has no unique Axiom IR v0.1 artifact")
	}
	irBytes, err := verified.ReadArtifact(irArtifact.ContentDigest)
	if err != nil {
		if code, ok := rejection.CodeOf(err); ok && code != rejection.ResourceLimit {
			evaluation, materializeErr := earlyIdentityEvaluation(
				verified, invocationBoundary, document, CheckRejected, err,
			)
			if materializeErr != nil {
				return Invocation{}, materializeErr
			}
			return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
		}
		return Invocation{}, err
	}
	ir, err := axiomir.ParseStructure(irBytes, invocationJSONLimits(verified.Request, ledger))
	if err != nil {
		if code, ok := rejection.CodeOf(err); ok && code != rejection.ResourceLimit {
			evaluation, materializeErr := earlySubjectEvaluation(
				verified, invocationBoundary, document, irArtifact.ContentDigest, err,
			)
			if materializeErr != nil {
				return Invocation{}, materializeErr
			}
			return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
		}
		return Invocation{}, err
	}
	subjectErr := document.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest)
	if subjectErr != nil {
		evaluation, err := earlySubjectEvaluation(
			verified, invocationBoundary, document, ir.ContentDigest, subjectErr,
		)
		if err != nil {
			return Invocation{}, err
		}
		return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
	}

	if resourceErr := ledger.Activate(); resourceErr != nil {
		evaluation, err := earlyResourceEvaluation(
			verified, invocationBoundary, document, ir.ContentDigest, resourceErr,
		)
		if err != nil {
			return Invocation{}, err
		}
		return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
	}

	evaluation, err := evaluateVerifiedBundle(verified, boundary, ledger)
	if err != nil {
		if code, ok := rejection.CodeOf(err); !ok || code != rejection.ResourceLimit {
			return Invocation{}, err
		}
		evaluation, err = earlyResourceEvaluation(
			verified, invocationBoundary, document, ir.ContentDigest, err,
		)
		if err != nil {
			return Invocation{}, err
		}
	}
	return encodeInvocation(evaluation, runtimeIdentity, ledger, verified.Request.AssurancePolicy.AllowedTrustCategories)
}

func RecordInvocationFailure(
	requestRaw []byte,
	limits strictjson.Limits,
) (Invocation, error) {
	failure, err := NewInvocationFailure(requestRaw, limits)
	if err != nil {
		return Invocation{}, err
	}
	return Invocation{Kind: InvocationFailed, Failure: failure}, nil
}

func encodeInvocation(
	evaluation Evaluation,
	runtimeIdentity RuntimeIdentity,
	ledger *resourcebudget.Ledger,
	allowedTrustCategories []string,
) (Invocation, error) {
	if err := ledger.Checkpoint(); err != nil {
		var materializeErr error
		evaluation, materializeErr = materializeEncodingResource(evaluation, allowedTrustCategories)
		if materializeErr != nil {
			return Invocation{}, materializeErr
		}
	}
	document, err := EncodeCompanion(evaluation.Result, runtimeIdentity)
	if err != nil {
		return Invocation{}, err
	}
	return Invocation{
		Kind: InvocationResult, Companion: document, Evaluation: evaluation,
		Resources: ledger.Snapshot(),
	}, nil
}

func materializeEncodingResource(
	evaluation Evaluation,
	allowedTrustCategories []string,
) (Evaluation, error) {
	incomplete, err := NewCheck(
		CheckIsolation, CheckIncomplete, []string{"tcb-incomplete"},
		[]Ref{{Kind: RefArtifact, ID: evaluation.Result.Boundary.Checker.Source}},
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks := append([]Check(nil), evaluation.Result.Checks...)
	replaced := false
	for index := range checks {
		if checks[index].Definition.Kind == CheckIsolation {
			checks[index] = incomplete
			replaced = true
			break
		}
	}
	if !replaced {
		checks = append(checks, incomplete)
	}
	result, err := Aggregate(Input{
		AllowedTrustCategories: allowedTrustCategories,
		Boundary:               evaluation.Result.Boundary,
		Checks:                 checks,
		MissingArtifacts:       evaluation.Result.MissingArtifacts,
		RemainingTrust:         evaluation.Result.RemainingTrust,
	})
	if err != nil {
		return Evaluation{}, err
	}
	evaluation.Result = result
	return evaluation, nil
}

func earlyIdentityEvaluation(
	verified bundle.Verified,
	boundary IdentityBoundary,
	document axiomevidence.Document,
	outcome CheckOutcome,
	finding error,
) (Evaluation, error) {
	code, ok := rejection.CodeOf(finding)
	if !ok {
		return Evaluation{}, finding
	}
	identity, err := NewCheck(
		CheckIdentity, outcome, []string{string(code)},
		[]Ref{{Kind: RefRequest, ID: verified.RequestDigest}},
	)
	if err != nil {
		return Evaluation{}, err
	}
	return aggregateEarly(verified, boundary, document, []Check{identity})
}

func earlySubjectEvaluation(
	verified bundle.Verified,
	boundary IdentityBoundary,
	document axiomevidence.Document,
	irContent protocol.Digest,
	finding error,
) (Evaluation, error) {
	strict, identity, err := passedPreflightChecks(verified, document)
	if err != nil {
		return Evaluation{}, err
	}
	subject, err := checkFromError(
		CheckSubject, "invalid-ir", []Ref{{Kind: RefArtifact, ID: irContent}}, finding,
	)
	if err != nil {
		return Evaluation{}, err
	}
	return aggregateEarly(verified, boundary, document, []Check{strict, identity, subject})
}

func earlyResourceEvaluation(
	verified bundle.Verified,
	boundary IdentityBoundary,
	document axiomevidence.Document,
	irContent protocol.Digest,
	resourceErr error,
) (Evaluation, error) {
	strict, identity, err := passedPreflightChecks(verified, document)
	if err != nil {
		return Evaluation{}, err
	}
	subject, err := NewCheck(
		CheckSubject, CheckPassed, []string{"subject-mismatch"},
		[]Ref{{Kind: RefArtifact, ID: irContent}},
	)
	if err != nil {
		return Evaluation{}, err
	}
	obligation, err := checkFromError(
		CheckObligation, "obligation-mismatch",
		[]Ref{{Kind: RefArtifact, ID: document.ContentDigest}}, resourceErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	return aggregateEarly(verified, boundary, document, []Check{strict, identity, subject, obligation})
}

func passedPreflightChecks(
	verified bundle.Verified,
	document axiomevidence.Document,
) (Check, Check, error) {
	strict, err := NewCheck(
		CheckStrictParse, CheckPassed, []string{"noncanonical-json"},
		[]Ref{{Kind: RefArtifact, ID: document.ContentDigest}},
	)
	if err != nil {
		return Check{}, Check{}, err
	}
	identity, err := NewCheck(
		CheckIdentity, CheckPassed, []string{"manifest-coverage"},
		[]Ref{{Kind: RefRequest, ID: verified.RequestDigest}},
	)
	return strict, identity, err
}

func aggregateEarly(
	verified bundle.Verified,
	boundary IdentityBoundary,
	document axiomevidence.Document,
	checks []Check,
) (Evaluation, error) {
	trust := trustInventory(document)
	result, err := Aggregate(Input{
		AllowedTrustCategories: verified.Request.AssurancePolicy.AllowedTrustCategories,
		Boundary:               boundary, Checks: checks, MissingArtifacts: verified.MissingArtifacts,
		RemainingTrust: trust,
	})
	if err != nil {
		return Evaluation{}, err
	}
	return Evaluation{Result: result}, nil
}

func invocationJSONLimits(request protocol.Request, ledger *resourcebudget.Ledger) strictjson.Limits {
	artifactBytes, _ := request.Limit("artifact-bytes")
	return strictjson.Limits{
		MaxBytes: artifactBytes, MaxDepth: 128, MaxItems: 10_000,
		MaxSteps: 1_000_000, Counter: ledger,
	}
}

func validateSourceBoundary(boundary IdentityBoundary) error {
	probe := boundary
	probe.Evidence = DocumentIdentity{
		ContentDigest: boundary.Checker.Source,
		DomainDigest:  boundary.Checker.Source, DomainAvailable: true,
	}
	probe.Request = probe.Evidence
	_, err := normalizeBoundary(probe)
	return err
}
