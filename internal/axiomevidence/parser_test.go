package axiomevidence_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/axiomevidence"
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/bundle"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func TestParseStructureImportedTwentyEightBundleBoundary(t *testing.T) {
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 28 {
		t.Fatalf("expected 28 imported scenarios, got %d", len(entries))
	}
	rejections := map[string]rejection.Code{
		"chk-bundle-01":   rejection.ArtifactMissing,
		"chk-digest-01":   rejection.DigestMismatch,
		"chk-resource-01": rejection.ResourceLimit,
	}
	parsed := 0
	complete := 0
	stateComplete := 0
	worldComplete := 0
	inputComplete := 0
	failedInputs := 0
	targetComplete := 0
	replayedProofs := 0
	deferredComparisons := 0
	outputComplete := 0
	hostExecutions := 0
	checkedComparisons := 0
	failedComparisons := 0
	replayedMismatches := 0
	proofComplete := 0
	proofClaims := 0
	proofVerified := 0
	proofAttestations := 0
	proofUnsupported := 0
	proofPolicySatisfied := 0
	proofMissingMaterial := 0
	conclusionComplete := 0
	conclusionKinds := make(map[string]int)
	uniqueEvidence := make(map[protocol.Digest]struct{})
	uniqueIR := make(map[protocol.Digest]struct{})
	uniqueInputs := make(map[protocol.Digest]struct{})
	uniqueOutputs := make(map[protocol.Digest]struct{})
	for _, entry := range entries {
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			scenarioRoot := filepath.Join(root, name)
			verified, err := bundle.Verify(filepath.Join(scenarioRoot, "bundle"))
			if want, rejected := rejections[name]; rejected {
				assertCode(t, err, want)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			evidenceArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-evidence", "0.1")
			if !ok {
				t.Fatal("bundle does not contain exactly one Axiom Evidence v0.1 artifact")
			}
			evidenceBytes := readBlob(t, scenarioRoot, evidenceArtifact.ContentDigest)
			document, err := axiomevidence.ParseStructure(evidenceBytes, limitsFrom(verified.Request))
			if err != nil {
				t.Fatal(err)
			}
			if document.ContentDigest != evidenceArtifact.ContentDigest {
				t.Fatal("Evidence parser content digest differs from verified bundle identity")
			}
			expectedDomain := expectedEvidenceDomainDigest(t, name, scenarioRoot)
			if err := document.VerifyDomainDigest(expectedDomain); err != nil {
				t.Fatal(err)
			}

			irArtifact, ok := uniqueArtifact(verified.Manifest, "axiom-ir", "0.1")
			if !ok {
				t.Fatal("bundle does not contain exactly one Axiom IR v0.1 artifact")
			}
			irBytes := readBlob(t, scenarioRoot, irArtifact.ContentDigest)
			irDocument, err := axiomir.ParseStructure(irBytes, limitsFrom(verified.Request))
			if err != nil {
				t.Fatal(err)
			}
			if err := document.VerifyIRSubject(irDocument.ContentDigest, irDocument.DomainDigest); err != nil {
				t.Fatal(err)
			}
			completenessErr := document.VerifyObligationCompleteness(irDocument)
			if name == "chk-obligation-01" {
				assertCode(t, completenessErr, rejection.ObligationMismatch)
			} else if completenessErr != nil {
				t.Fatal(completenessErr)
			} else {
				complete++
				if err := document.VerifyStateSupport(); err != nil {
					t.Fatal(err)
				}
				stateComplete++
				if err := document.VerifyCounterexampleWorlds(irDocument); err != nil {
					t.Fatal(err)
				}
				worldComplete++
				inputCheck, err := document.VerifyConcreteInputs(
					irDocument, verified.ReadArtifact, concreteLimitsFrom(verified.Request),
				)
				if err != nil {
					t.Fatal(err)
				}
				inputComplete++
				failedInputs += inputCheck.Failed
				for _, artifact := range inputCheck.Artifacts {
					uniqueInputs[artifact] = struct{}{}
				}
				targetCheck, err := document.VerifyCounterexampleTargets(
					irDocument, counterexampleLimitsFrom(verified.Request),
				)
				if err != nil {
					t.Fatal(err)
				}
				targetComplete++
				replayedProofs += targetCheck.ReplayedProofs
				deferredComparisons += targetCheck.DeferredComparisons
				outputCheck, err := document.VerifyConcreteOutputs(
					irDocument, verified.ReadArtifact, concreteLimitsFrom(verified.Request),
				)
				if err != nil {
					t.Fatal(err)
				}
				outputComplete++
				hostExecutions += outputCheck.HostExecutions
				checkedComparisons += outputCheck.CheckedComparisons
				failedComparisons += outputCheck.FailedComparisons
				replayedMismatches += outputCheck.ReplayedMismatches
				for _, artifact := range outputCheck.Artifacts {
					uniqueOutputs[artifact] = struct{}{}
				}
				proofCheck, err := document.InspectProofSupports(
					verified.Request.AssurancePolicy,
					verified.ReadArtifact,
					concreteLimitsFrom(verified.Request),
				)
				if err != nil {
					t.Fatal(err)
				}
				proofComplete++
				proofClaims += proofCheck.Claims
				proofVerified += proofCheck.IndependentlyVerified
				proofAttestations += proofCheck.AttestationsConfirmed
				proofUnsupported += proofCheck.UnsupportedClaims
				proofPolicySatisfied += proofCheck.ProofPolicySatisfied
				proofMissingMaterial += proofCheck.MissingProofMaterial
				conclusionCheck, err := document.VerifyConclusion(conclusionLimitsFrom(verified.Request))
				if err != nil {
					t.Fatal(err)
				}
				conclusionComplete++
				conclusionKinds[conclusionCheck.Kind]++
			}
			if document.Counts.Artifacts == 0 || document.Counts.Executions == 0 ||
				document.Counts.Obligations == 0 || document.Counts.Tools == 0 {
				t.Fatal("Evidence parser omitted a required top-level entry collection")
			}
			uniqueEvidence[document.ContentDigest] = struct{}{}
			uniqueIR[irDocument.ContentDigest] = struct{}{}
			parsed++
		})
	}
	if parsed != 25 {
		t.Fatalf("expected 25 identity-valid Evidence scenarios, got %d", parsed)
	}
	if complete != 24 {
		t.Fatalf("expected 24 obligation-complete Evidence scenarios, got %d", complete)
	}
	if stateComplete != 24 {
		t.Fatalf("expected 24 state/support-complete Evidence scenarios, got %d", stateComplete)
	}
	if worldComplete != 24 {
		t.Fatalf("expected 24 counterexample-world-complete Evidence scenarios, got %d", worldComplete)
	}
	if inputComplete != 24 {
		t.Fatalf("expected 24 concrete-input-complete Evidence scenarios, got %d", inputComplete)
	}
	if len(uniqueInputs) != 12 {
		t.Fatalf("expected 12 unique host-input artifacts, got %d", len(uniqueInputs))
	}
	if failedInputs != 4 {
		t.Fatalf("expected four failed invalid-input classifications, got %d", failedInputs)
	}
	if targetComplete != 24 {
		t.Fatalf("expected 24 counterexample-target-complete Evidence scenarios, got %d", targetComplete)
	}
	if replayedProofs != 8 {
		t.Fatalf("expected eight replayed failed proof targets, got %d", replayedProofs)
	}
	if deferredComparisons != 3 {
		t.Fatalf("expected three deferred host/output comparisons, got %d", deferredComparisons)
	}
	if outputComplete != 24 {
		t.Fatalf("expected 24 concrete-output-complete Evidence scenarios, got %d", outputComplete)
	}
	if len(uniqueOutputs) != 8 {
		t.Fatalf("expected eight unique concrete output artifacts, got %d", len(uniqueOutputs))
	}
	if proofComplete != 24 || proofClaims != 213 {
		t.Fatalf("expected 24 proof audits and 213 proved claims, got %d and %d", proofComplete, proofClaims)
	}
	if proofVerified != 0 || proofAttestations != 65 || proofUnsupported != 148 {
		t.Fatalf(
			"unexpected proof capability classification: verified=%d attestations=%d unsupported=%d",
			proofVerified, proofAttestations, proofUnsupported,
		)
	}
	if proofPolicySatisfied != 53 || proofMissingMaterial != 160 {
		t.Fatalf(
			"unexpected proof policy boundary: satisfied=%d missing=%d",
			proofPolicySatisfied, proofMissingMaterial,
		)
	}
	if conclusionComplete != 24 ||
		conclusionKinds[axiomevidence.ConclusionSatisfied] != 7 ||
		conclusionKinds[axiomevidence.ConclusionInputRejected] != 4 ||
		conclusionKinds[axiomevidence.ConclusionViolated] != 8 ||
		conclusionKinds[axiomevidence.ConclusionInconclusive] != 4 ||
		conclusionKinds[axiomevidence.ConclusionImplementationInconsistent] != 1 {
		t.Fatalf("unexpected independently recomputed conclusion distribution: %+v", conclusionKinds)
	}
	if failedComparisons != 1 || replayedMismatches != 3 {
		t.Fatalf(
			"expected one failed comparison and three replayed mismatch obligations, got %d and %d",
			failedComparisons, replayedMismatches,
		)
	}
	if hostExecutions != 9 || checkedComparisons != 7 {
		t.Fatalf(
			"expected nine host executions and seven checked comparisons, got %d and %d",
			hostExecutions, checkedComparisons,
		)
	}
	if len(uniqueEvidence) != 25 {
		t.Fatalf("expected 25 unique identity-valid Evidence documents, got %d", len(uniqueEvidence))
	}
	if len(uniqueIR) != 12 {
		t.Fatalf("expected 12 unique identity-valid IR documents, got %d", len(uniqueIR))
	}
}

func concreteLimitsFrom(request protocol.Request) axiomevidence.ConcreteInputLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomevidence.ConcreteInputLimits{
		JSON:             limitsFrom(request),
		MaxLogicalBytes:  workingMemory,
		MaxSemanticSteps: semanticSteps,
	}
}

func counterexampleLimitsFrom(request protocol.Request) axiomir.ExecutionLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomir.ExecutionLimits{
		MaxLogicalBytes:  workingMemory,
		MaxSemanticSteps: semanticSteps,
	}
}

func conclusionLimitsFrom(request protocol.Request) axiomevidence.ConclusionLimits {
	workingMemory, _ := request.Limit("working-memory")
	semanticSteps, _ := request.Limit("semantic-steps")
	return axiomevidence.ConclusionLimits{
		MaxLogicalBytes:  workingMemory,
		MaxSemanticSteps: semanticSteps,
	}
}

func TestParseStructureRejectsClosedBoundaryViolations(t *testing.T) {
	data := lockedEvidence(t, "ax-b01-correct")
	tests := []struct {
		name   string
		mutate func(*testing.T, []byte) []byte
		code   rejection.Code
	}{
		{
			"unknown top-level member",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input, []byte(`"format":"axiom-evidence"`), []byte(`"formaz":"axiom-evidence"`))
			},
			rejection.UnknownMember,
		},
		{
			"unsupported version",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input, []byte(`"evidence_version":"0.1"`), []byte(`"evidence_version":"0.2"`))
			},
			rejection.UnsupportedVersion,
		},
		{
			"unknown support tag",
			func(t *testing.T, input []byte) []byte {
				return replaceFirst(t, input, []byte(`"kind":"backend-attestation"`), []byte(`"kind":"backend-assertion"`))
			},
			rejection.UnknownTag,
		},
		{
			"noncanonical artifact order",
			func(t *testing.T, input []byte) []byte {
				return swapFirstTwoObjects(t, input, "artifacts")
			},
			rejection.NoncanonicalOrder,
		},
		{
			"noncanonical obligation order",
			func(t *testing.T, input []byte) []byte {
				return swapFirstTwoObjects(t, input, "obligations")
			},
			rejection.NoncanonicalOrder,
		},
		{
			"tool definition domain ID drift",
			func(t *testing.T, input []byte) []byte {
				start := bytes.Index(input, []byte(`"tools":[`))
				if start < 0 {
					t.Fatal("tools array not found")
				}
				return mutateDigestAfter(t, input, start, []byte(`"id":"sha256:`))
			},
			rejection.DigestMismatch,
		},
		{
			"dangling producer",
			func(t *testing.T, input []byte) []byte {
				start := bytes.Index(input, []byte(`"producer":"sha256:`))
				if start < 0 {
					t.Fatal("producer not found")
				}
				return zeroDigestAfter(t, input, start, []byte(`"producer":"sha256:`))
			},
			rejection.InvalidJSON,
		},
		{
			"subject artifact is not IR",
			func(t *testing.T, input []byte) []byte {
				return replaceOnce(t, input,
					[]byte(`"ir_artifact":"sha256:8d6ec839cf3cf795539e121c488b3bc4be84a33f24ac8c349d5ceebb29e559e7"`),
					[]byte(`"ir_artifact":"sha256:be23a85c7243960346f95c1c6b8efbf74d40c6cac812333389519f239c6249e"`),
				)
			},
			rejection.InvalidJSON,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := axiomevidence.ParseStructure(test.mutate(t, data), fixtureLimits())
			assertCode(t, err, test.code)
		})
	}
}

func TestParseStructureRejectsDuplicateConclusionReference(t *testing.T) {
	data := lockedEvidence(t, "ax-b01-backend-timeout")
	mutated := duplicateSecondDigestInArray(t, data, []byte(`"conclusion":`), []byte(`"refs":[`))
	_, err := axiomevidence.ParseStructure(mutated, fixtureLimits())
	assertCode(t, err, rejection.NoncanonicalOrder)
}

func TestVerifyConclusionRejectsTamperedProducerBytes(t *testing.T) {
	data := lockedEvidence(t, "ax-b01-correct")
	mutated := replaceOnce(t, data,
		[]byte(`"conclusion":{"kind":"satisfied","refs":[]}`),
		[]byte(`"conclusion":{"kind":"violated","refs":[]}`),
	)
	document, err := axiomevidence.ParseStructure(mutated, fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	_, err = document.VerifyConclusion(axiomevidence.ConclusionLimits{
		MaxLogicalBytes: 1 << 20, MaxSemanticSteps: 1_000_000,
	})
	assertCode(t, err, rejection.ConclusionMismatch)
}

func TestDocumentBindingsRejectMismatch(t *testing.T) {
	document, err := axiomevidence.ParseStructure(lockedEvidence(t, "ax-b01-correct"), fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.VerifyDomainDigest(protocol.Digest{}); err == nil {
		t.Fatal("expected Evidence document domain digest mismatch")
	} else {
		assertCode(t, err, rejection.DigestMismatch)
	}
	if err := document.VerifyIRSubject(protocol.Digest{}, protocol.Digest{}); err == nil {
		t.Fatal("expected Evidence subject binding mismatch")
	} else {
		assertCode(t, err, rejection.DigestMismatch)
	}
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

func readBlob(t *testing.T, scenarioRoot string, digest protocol.Digest) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scenarioRoot, "bundle", "blobs", "sha256", digest.BlobName()))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func lockedEvidence(t *testing.T, scenario string) []byte {
	t.Helper()
	root := filepath.Join("..", "bundle", "testdata", "upstream", "s", scenario)
	verified, err := bundle.Verify(filepath.Join(root, "bundle"))
	if err != nil {
		t.Fatal(err)
	}
	artifact, ok := uniqueArtifact(verified.Manifest, "axiom-evidence", "0.1")
	if !ok {
		t.Fatal("fixture does not contain exactly one Axiom Evidence artifact")
	}
	return readBlob(t, root, artifact.ContentDigest)
}

func expectedEvidenceDomainDigest(t *testing.T, scenario, scenarioRoot string) protocol.Digest {
	t.Helper()
	if scenario == "chk-process-01" {
		// The process-failure scenario intentionally has no independent result.
		// Its Evidence identity is locked by the upstream bundle-set manifest.
		digest, err := protocol.ParseDigest("sha256:2412e0933ea7fe5359b7bf7819204877250c1b947c8498955eaf242be24c000c")
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	type expectedResult struct {
		Evidence struct {
			DocumentDigest struct {
				Kind  string `json:"kind"`
				Value string `json:"value"`
			} `json:"document_digest"`
		} `json:"evidence"`
	}
	data, err := os.ReadFile(filepath.Join(scenarioRoot, "expected-result.jcs"))
	if err != nil {
		t.Fatal(err)
	}
	var expected expectedResult
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if expected.Evidence.DocumentDigest.Kind != "available" {
		t.Fatalf("expected locked Evidence document digest, got %q", expected.Evidence.DocumentDigest.Kind)
	}
	digest, err := protocol.ParseDigest(expected.Evidence.DocumentDigest.Value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func limitsFrom(request protocol.Request) strictjson.Limits {
	artifactBytes, _ := request.Limit("artifact-bytes")
	depth, _ := request.Limit("json-depth")
	items, _ := request.Limit("collection-items")
	steps, _ := request.Limit("semantic-steps")
	return strictjson.Limits{MaxBytes: artifactBytes, MaxDepth: depth, MaxItems: items, MaxSteps: steps}
}

func fixtureLimits() strictjson.Limits {
	return strictjson.Limits{MaxBytes: 1 << 20, MaxDepth: 128, MaxItems: 10_000, MaxSteps: 1_000_000}
}

func replaceOnce(t *testing.T, input, old, replacement []byte) []byte {
	t.Helper()
	if bytes.Count(input, old) != 1 {
		t.Fatalf("expected one replacement target, got %d", bytes.Count(input, old))
	}
	return bytes.Replace(input, old, replacement, 1)
}

func replaceFirst(t *testing.T, input, old, replacement []byte) []byte {
	t.Helper()
	if bytes.Count(input, old) == 0 {
		t.Fatal("replacement target not found")
	}
	return bytes.Replace(input, old, replacement, 1)
}

func mutateDigestAfter(t *testing.T, input []byte, start int, marker []byte) []byte {
	t.Helper()
	result := append([]byte(nil), input...)
	index := bytes.Index(result[start:], marker)
	if index < 0 {
		t.Fatal("digest marker not found")
	}
	index += start + len(marker)
	if result[index] == '0' {
		result[index] = '1'
	} else {
		result[index] = '0'
	}
	return result
}

func zeroDigestAfter(t *testing.T, input []byte, start int, marker []byte) []byte {
	t.Helper()
	result := append([]byte(nil), input...)
	index := bytes.Index(result[start:], marker)
	if index < 0 {
		t.Fatal("digest marker not found")
	}
	index += start + len(marker)
	copy(result[index:index+64], bytes.Repeat([]byte{'0'}, 64))
	return result
}

func swapFirstTwoObjects(t *testing.T, input []byte, arrayName string) []byte {
	t.Helper()
	marker := []byte(`"` + arrayName + `":[`)
	start := bytes.Index(input, marker)
	if start < 0 {
		t.Fatal("array marker not found")
	}
	firstStart := start + len(marker)
	firstEnd := objectEnd(t, input, firstStart)
	if firstEnd+1 >= len(input) || input[firstEnd+1] != ',' {
		t.Fatal("array does not contain two leading objects")
	}
	secondStart := firstEnd + 2
	secondEnd := objectEnd(t, input, secondStart)
	result := make([]byte, 0, len(input))
	result = append(result, input[:firstStart]...)
	result = append(result, input[secondStart:secondEnd+1]...)
	result = append(result, ',')
	result = append(result, input[firstStart:firstEnd+1]...)
	result = append(result, input[secondEnd+1:]...)
	return result
}

func objectEnd(t *testing.T, input []byte, start int) int {
	t.Helper()
	if start >= len(input) || input[start] != '{' {
		t.Fatal("object does not start at expected offset")
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(input); index++ {
		b := input[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	t.Fatal("unterminated object")
	return -1
}

func duplicateSecondDigestInArray(t *testing.T, input, sectionMarker, arrayMarker []byte) []byte {
	t.Helper()
	section := bytes.Index(input, sectionMarker)
	if section < 0 {
		t.Fatal("section marker not found")
	}
	arrayOffset := bytes.Index(input[section:], arrayMarker)
	if arrayOffset < 0 {
		t.Fatal("array marker not found")
	}
	start := section + arrayOffset + len(arrayMarker)
	firstQuote := bytes.IndexByte(input[start:], '"')
	if firstQuote != 0 {
		t.Fatal("first digest does not start at array head")
	}
	firstEnd := bytes.IndexByte(input[start+1:], '"')
	if firstEnd < 0 {
		t.Fatal("first digest is unterminated")
	}
	firstEnd += start + 1
	secondStart := bytes.IndexByte(input[firstEnd+1:], '"')
	if secondStart < 0 {
		t.Fatal("second digest is missing")
	}
	secondStart += firstEnd + 1
	secondEnd := bytes.IndexByte(input[secondStart+1:], '"')
	if secondEnd < 0 {
		t.Fatal("second digest is unterminated")
	}
	secondEnd += secondStart + 1
	first := input[start+1 : firstEnd]
	if len(first) != secondEnd-secondStart-1 {
		t.Fatal("digest lengths differ")
	}
	result := append([]byte(nil), input...)
	copy(result[secondStart+1:secondEnd], first)
	return result
}

func assertCode(t *testing.T, err error, want rejection.Code) {
	t.Helper()
	if got, ok := rejection.CodeOf(err); err == nil || !ok || got != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
