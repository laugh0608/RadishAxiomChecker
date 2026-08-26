package checkresult

import (
	"fmt"
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/axiomevidence"
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// Evaluation retains the real semantic findings that determined Result. The
// producer conclusion remains separate from the independent four-state result,
// and proof gaps remain inspectable even when an earlier producer conclusion is
// decisive.
type Evaluation struct {
	Result       Result
	Conclusion   axiomevidence.ConclusionCheck
	ProofSupport axiomevidence.ProofSupportCheck
}

// EvaluateVerifiedBundle runs the current result-layer checks over a bundle
// whose request, manifest, present blobs, and missing-blob inventory have
// already been established by bundle.Verify. It forms only an in-memory result;
// it does not encode a companion document or claim a checker binary identity.
func EvaluateVerifiedBundle(verified bundle.Verified, runtimeBoundary IdentityBoundary) (Evaluation, error) {
	evidenceArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-evidence", "0.1")
	if !ok {
		return Evaluation{}, fmt.Errorf("verified bundle does not contain one Axiom Evidence v0.1 artifact")
	}
	irArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-ir", "0.1")
	if !ok {
		return Evaluation{}, fmt.Errorf("verified bundle does not contain one Axiom IR v0.1 artifact")
	}
	evidenceBytes, err := verified.ReadArtifact(evidenceArtifact.ContentDigest)
	if err != nil {
		return Evaluation{}, fmt.Errorf("Evidence bytes are unavailable for result-layer evaluation: %w", err)
	}
	document, err := axiomevidence.ParseStructure(evidenceBytes, jsonLimits(verified.Request))
	if err != nil {
		return Evaluation{}, fmt.Errorf("Evidence did not enter the in-memory result layer: %w", err)
	}
	irBytes, err := verified.ReadArtifact(irArtifact.ContentDigest)
	if err != nil {
		return Evaluation{}, fmt.Errorf("IR bytes are unavailable for result-layer evaluation: %w", err)
	}
	ir, err := axiomir.ParseStructure(irBytes, jsonLimits(verified.Request))
	if err != nil {
		return Evaluation{}, fmt.Errorf("IR did not enter the in-memory result layer: %w", err)
	}

	boundary := runtimeBoundary
	boundary.Evidence = DocumentIdentity{
		ContentDigest: document.ContentDigest, DomainDigest: document.DomainDigest, DomainAvailable: true,
	}
	boundary.Request = DocumentIdentity{
		ContentDigest: verified.RequestDigest, DomainDigest: verified.Request.DomainDigest, DomainAvailable: true,
	}

	checks := make([]Check, 0, len(requiredCheckKinds))
	strictCheck, err := NewCheck(CheckStrictParse, CheckPassed, []string{"noncanonical-json"}, []Ref{
		{Kind: RefArtifact, ID: document.ContentDigest},
	})
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, strictCheck)

	identityOutcome := CheckPassed
	identityCode := "manifest-coverage"
	identityRefs := []Ref{
		{Kind: RefArtifact, ID: document.ContentDigest},
		{Kind: RefArtifact, ID: ir.ContentDigest},
		{Kind: RefArtifact, ID: verified.ManifestDigest},
		{Kind: RefRequest, ID: verified.RequestDigest},
	}
	if len(verified.MissingArtifacts) != 0 {
		identityOutcome = CheckIncomplete
		identityCode = "artifact-missing"
		for _, artifact := range verified.MissingArtifacts {
			identityRefs = append(identityRefs, Ref{Kind: RefArtifact, ID: artifact})
		}
	}
	identityCheck, err := NewCheck(CheckIdentity, identityOutcome, []string{identityCode}, uniqueRefs(identityRefs))
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, identityCheck)

	subjectErr := document.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest)
	subjectCheck, err := checkFromError(
		CheckSubject, "subject-mismatch", []Ref{{Kind: RefArtifact, ID: ir.ContentDigest}}, subjectErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, subjectCheck)

	obligationErr := document.VerifyObligationCompleteness(ir)
	obligationCheck, err := checkFromError(
		CheckObligation, "obligation-mismatch", []Ref{{Kind: RefArtifact, ID: document.ContentDigest}}, obligationErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, obligationCheck)

	trust := trustInventory(document)
	stateErr := document.VerifyStateSupport()
	stateOutcome := CheckPassed
	stateCode := "invalid-state-support"
	stateRefs := []Ref{{Kind: RefArtifact, ID: document.ContentDigest}}
	if stateErr != nil {
		stateOutcome = outcomeForError(stateErr)
	} else {
		disallowed := disallowedTrust(trust, verified.Request.AssurancePolicy.AllowedTrustCategories)
		switch {
		case len(disallowed) != 0:
			stateOutcome = CheckIncomplete
			stateCode = "trust-not-allowed"
			stateRefs = trustRefs(disallowed)
		case len(trust) != 0:
			stateOutcome = CheckTrusted
			stateRefs = trustRefs(trust)
		}
	}
	stateCheck, err := NewCheck(CheckStateSupport, stateOutcome, []string{stateCode}, uniqueRefs(stateRefs))
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, stateCheck)

	counterexampleErr := document.VerifyCounterexampleWorlds(ir)
	if counterexampleErr == nil {
		_, counterexampleErr = document.VerifyCounterexampleTargets(ir, executionLimits(verified.Request))
	}
	counterexampleCheck, err := checkFromError(
		CheckCounterexampleReplay, "counterexample-invalid",
		[]Ref{{Kind: RefArtifact, ID: document.ContentDigest}}, counterexampleErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, counterexampleCheck)

	concreteRefs := make([]Ref, 0)
	inputCheck, concreteErr := document.VerifyConcreteInputs(
		ir, verified.ReadArtifact, concreteLimits(verified.Request),
	)
	if concreteErr == nil {
		for _, artifact := range inputCheck.Artifacts {
			concreteRefs = append(concreteRefs, Ref{Kind: RefArtifact, ID: artifact})
		}
	}
	outputCheck := axiomevidence.ConcreteOutputCheck{}
	if concreteErr == nil {
		outputCheck, concreteErr = document.VerifyConcreteOutputs(
			ir, verified.ReadArtifact, concreteLimits(verified.Request),
		)
		for _, artifact := range outputCheck.Artifacts {
			concreteRefs = append(concreteRefs, Ref{Kind: RefArtifact, ID: artifact})
		}
	}
	if len(concreteRefs) == 0 {
		concreteRefs = append(concreteRefs, Ref{Kind: RefArtifact, ID: document.ContentDigest})
	}
	concreteCode := "concrete-check-mismatch"
	if concreteErr == nil && (outputCheck.FailedComparisons != 0 || outputCheck.ReplayedMismatches != 0) {
		concreteCode = "host-output-mismatch"
	}
	concreteCheck, err := checkFromError(
		CheckConcreteReplay, concreteCode, uniqueRefs(concreteRefs), concreteErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, concreteCheck)

	proof, proofErr := document.InspectProofSupports(
		verified.Request.AssurancePolicy, verified.ReadArtifact, concreteLimits(verified.Request),
	)
	conclusion, conclusionErr := document.VerifyConclusion(conclusionLimits(verified.Request))

	proofCheck, err := materializeProofCheck(document.ContentDigest, verified.Request.AssurancePolicy, proof, proofErr, conclusion)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, proofCheck)

	conclusionRefs := make([]Ref, 0, len(conclusion.Refs))
	for _, ref := range conclusion.Refs {
		kind := RefObligation
		if ref.Kind == axiomevidence.ConclusionRefExecution {
			kind = RefEvidenceEntry
		}
		conclusionRefs = append(conclusionRefs, Ref{Kind: kind, ID: ref.Value})
	}
	if len(conclusionRefs) == 0 {
		conclusionRefs = append(conclusionRefs, Ref{Kind: RefArtifact, ID: document.ContentDigest})
	}
	conclusionCheck, err := checkFromError(
		CheckConclusion, "conclusion-mismatch", uniqueRefs(conclusionRefs), conclusionErr,
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, conclusionCheck)

	isolationCheck, err := NewCheck(
		CheckIsolation, CheckPassed, []string{"isolation-boundary-violation"},
		[]Ref{{Kind: RefArtifact, ID: runtimeBoundary.Checker.Source}},
	)
	if err != nil {
		return Evaluation{}, err
	}
	checks = append(checks, isolationCheck)

	result, err := Aggregate(Input{
		AllowedTrustCategories: verified.Request.AssurancePolicy.AllowedTrustCategories,
		Boundary:               boundary,
		Checks:                 checks,
		MissingArtifacts:       verified.MissingArtifacts,
		RemainingTrust:         trust,
	})
	if err != nil {
		return Evaluation{}, err
	}
	return Evaluation{Result: result, Conclusion: conclusion, ProofSupport: proof}, nil
}

func materializeProofCheck(
	evidence protocol.Digest,
	policy protocol.AssurancePolicy,
	proof axiomevidence.ProofSupportCheck,
	proofErr error,
	conclusion axiomevidence.ConclusionCheck,
) (Check, error) {
	refs := make([]Ref, 0, len(proof.Findings)+len(proof.Artifacts)+len(proof.RemainingTrust))
	for _, finding := range proof.Findings {
		refs = append(refs, Ref{Kind: RefObligation, ID: finding.Obligation})
	}
	for _, artifact := range proof.Artifacts {
		refs = append(refs, Ref{Kind: RefArtifact, ID: artifact})
	}
	for _, trust := range proof.RemainingTrust {
		refs = append(refs, Ref{Kind: RefTrust, ID: trust})
	}
	if len(refs) == 0 {
		refs = append(refs, Ref{Kind: RefArtifact, ID: evidence})
	}
	outcome := CheckPassed
	code := "proof-support-mismatch"
	if proofErr != nil {
		outcome = outcomeForError(proofErr)
		code = "proof-support-unsupported"
	} else if conclusion.Kind == axiomevidence.ConclusionSatisfied && proof.MissingProofMaterial != 0 {
		outcome = CheckIncomplete
		code = "proof-support-unsupported"
		if policy.ProofSupport == "certificate-required" {
			code = "certificate-incomplete"
		}
	} else if len(proof.RemainingTrust) != 0 {
		outcome = CheckTrusted
	} else if proof.MissingProofMaterial != 0 {
		code = "proof-support-unsupported"
	}
	return NewCheck(CheckProofSupport, outcome, []string{code}, uniqueRefs(refs))
}

func checkFromError(kind CheckKind, code string, refs []Ref, err error) (Check, error) {
	outcome := CheckPassed
	if err != nil {
		outcome = outcomeForError(err)
	}
	return NewCheck(kind, outcome, []string{code}, uniqueRefs(refs))
}

func outcomeForError(err error) CheckOutcome {
	code, ok := rejection.CodeOf(err)
	if ok && (code == rejection.ResourceLimit || code == rejection.ArtifactMissing) {
		return CheckIncomplete
	}
	return CheckRejected
}

func uniqueArtifact(manifest protocol.Manifest, format, version string) (protocol.Artifact, bool) {
	var result protocol.Artifact
	found := false
	for _, artifact := range manifest.Artifacts {
		if artifact.Format != format || artifact.FormatVersion != version {
			continue
		}
		if found {
			return protocol.Artifact{}, false
		}
		result = artifact
		found = true
	}
	return result, found
}

func trustInventory(document axiomevidence.Document) []Trust {
	entries := document.TrustInventory()
	result := make([]Trust, 0, len(entries))
	for _, entry := range entries {
		result = append(result, Trust{ID: entry.ID, Category: entry.Category})
	}
	return result
}

func disallowedTrust(trust []Trust, categories []string) []Trust {
	allowed := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		allowed[category] = struct{}{}
	}
	result := make([]Trust, 0)
	for _, item := range trust {
		if _, ok := allowed[item.Category]; !ok {
			result = append(result, item)
		}
	}
	return result
}

func trustRefs(trust []Trust) []Ref {
	result := make([]Ref, 0, len(trust))
	for _, item := range trust {
		result = append(result, Ref{Kind: RefTrust, ID: item.ID})
	}
	return result
}

func uniqueRefs(input []Ref) []Ref {
	result := append([]Ref(nil), input...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].ID.String() < result[j].ID.String()
	})
	unique := result[:0]
	for _, ref := range result {
		if len(unique) == 0 || unique[len(unique)-1] != ref {
			unique = append(unique, ref)
		}
	}
	return unique
}

func jsonLimits(request protocol.Request) strictjson.Limits {
	artifactBytes, _ := request.Limit("artifact-bytes")
	depth, _ := request.Limit("json-depth")
	items, _ := request.Limit("collection-items")
	steps, _ := request.Limit("semantic-steps")
	return strictjson.Limits{MaxBytes: artifactBytes, MaxDepth: depth, MaxItems: items, MaxSteps: steps}
}

func concreteLimits(request protocol.Request) axiomevidence.ConcreteDataLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomevidence.ConcreteDataLimits{
		JSON: jsonLimits(request), MaxLogicalBytes: workingMemory, MaxSemanticSteps: semanticSteps,
	}
}

func executionLimits(request protocol.Request) axiomir.ExecutionLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomir.ExecutionLimits{MaxLogicalBytes: workingMemory, MaxSemanticSteps: semanticSteps}
}

func conclusionLimits(request protocol.Request) axiomevidence.ConclusionLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomevidence.ConclusionLimits{MaxLogicalBytes: workingMemory, MaxSemanticSteps: semanticSteps}
}
