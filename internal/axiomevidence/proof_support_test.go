package axiomevidence

import (
	"bytes"
	"crypto/sha256"
	"reflect"
	"testing"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestInspectProofSupportsClassifiesLockedCapabilityBoundary(t *testing.T) {
	tests := []struct {
		scenario       string
		claims         int
		attestations   int
		unsupported    int
		policyPassed   int
		missing        int
		artifacts      int
		remainingTrust int
	}{
		{"ax-b01-correct", 12, 12, 0, 12, 0, 4, 1},
		{"ax-b01-invalid-input", 12, 0, 12, 0, 12, 4, 0},
		{"chk-proof-01", 12, 12, 0, 0, 12, 4, 1},
		{"chk-proof-02", 12, 12, 0, 12, 0, 4, 1},
		{"ax-b01-backend-timeout", 0, 0, 0, 0, 0, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			document, _, verified := concreteInputFixture(t, test.scenario)
			check, err := document.InspectProofSupports(
				verified.Request.AssurancePolicy,
				verified.ReadArtifact,
				concreteInputLimits(verified.Request),
			)
			if err != nil {
				t.Fatal(err)
			}
			if check.Claims != test.claims ||
				check.AttestationsConfirmed != test.attestations ||
				check.UnsupportedClaims != test.unsupported ||
				check.ProofPolicySatisfied != test.policyPassed ||
				check.MissingProofMaterial != test.missing ||
				len(check.Artifacts) != test.artifacts ||
				len(check.RemainingTrust) != test.remainingTrust {
				t.Fatalf("unexpected proof support summary: %+v", check)
			}
			if check.IndependentlyVerified != 0 ||
				len(check.Capabilities.CertificateProfiles) != 0 ||
				len(check.Capabilities.KernelRuleProfiles) != 0 {
				t.Fatalf("locked empty capability set was silently expanded: %+v", check)
			}
			for _, finding := range check.Findings {
				if finding.QueryLogic != "QF_UFLIA" || finding.ResponseStatus != "unsat" ||
					!finding.TargetInObligationSet || finding.QueryTheoremVerified {
					t.Fatalf("query/status inspection was confused with theorem verification: %+v", finding)
				}
			}
		})
	}
}

func TestInspectProofSupportsDoesNotUpgradeSatKernelClaims(t *testing.T) {
	document, _, verified := concreteInputFixture(t, "ax-b01-wrong-add")
	check, err := document.InspectProofSupports(
		verified.Request.AssurancePolicy, verified.ReadArtifact, concreteInputLimits(verified.Request),
	)
	if err != nil {
		t.Fatal(err)
	}
	if check.Claims != 11 || check.UnsupportedClaims != 11 || check.MissingProofMaterial != 11 ||
		check.IndependentlyVerified != 0 || check.ProofPolicySatisfied != 0 {
		t.Fatalf("sat kernel claims were not failed closed: %+v", check)
	}
	for _, finding := range check.Findings {
		if finding.ResponseStatus != "sat" || finding.Coverage != ProofCoverageMissing ||
			finding.MissingReason != "kernel-replay-material-unavailable" {
			t.Fatalf("sat kernel claim was upgraded or obscured: %+v", finding)
		}
	}
}

func TestInspectProofSupportsRejectsBindingFormatAndStatusDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *Document, bundleReader) ArtifactReader
	}{
		{
			name: "obligation-set format drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				set, _, _ := proofExecutionArtifacts(t, document)
				definition := document.artifacts[set]
				definition.format = "axiom-obligations"
				document.artifacts[set] = definition
				return source.read
			},
		},
		{
			name: "obligation-set subject drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				other, _, otherVerified := concreteInputFixture(t, "ax-b02-correct")
				otherSet, _, _ := proofExecutionArtifacts(t, &other)
				data, err := otherVerified.ReadArtifact(otherSet)
				if err != nil {
					t.Fatal(err)
				}
				return rebindProofArtifact(t, document, source, "obligation-set", data)
			},
		},
		{
			name: "query logic drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				_, query, _ := proofExecutionArtifacts(t, document)
				data, err := source.read(query)
				if err != nil {
					t.Fatal(err)
				}
				data = bytes.Replace(data, []byte("QF_UFLIA"), []byte("QF_BV"), 1)
				return rebindProofArtifact(t, document, source, "query", data)
			},
		},
		{
			name: "duplicate status command",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				_, query, _ := proofExecutionArtifacts(t, document)
				data, err := source.read(query)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data, []byte("(check-sat)\n")...)
				return rebindProofArtifact(t, document, source, "query", data)
			},
		},
		{
			name: "response status drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				return rebindProofArtifact(t, document, source, "response", []byte("sat\n"))
			},
		},
		{
			name: "attestation trust scope drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				for id, result := range document.results {
					if result.kind != "proved" || result.support.kind != "backend-attestation" {
						continue
					}
					trust := document.trust[result.support.trust]
					trust.scopeTool = protocol.Digest{}
					document.trust[result.support.trust] = trust
					document.results[id] = result
					return source.read
				}
				t.Fatal("backend attestation not found")
				return nil
			},
		},
		{
			name: "proved execution binding drift",
			mutate: func(t *testing.T, document *Document, source bundleReader) ArtifactReader {
				for id, result := range document.results {
					if result.kind != "proved" {
						continue
					}
					result.support.execution = protocol.Digest{}
					document.results[id] = result
					return source.read
				}
				t.Fatal("proved result not found")
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, _, verified := concreteInputFixture(t, "ax-b01-correct")
			reader := test.mutate(t, &document, bundleReader{read: verified.ReadArtifact})
			_, err := document.InspectProofSupports(
				verified.Request.AssurancePolicy, reader, concreteInputLimits(verified.Request),
			)
			assertCompletenessCode(t, err, rejection.InvalidStateSupport)
		})
	}
}

func TestInspectProofSupportsRejectsIdentityAndResourceDrift(t *testing.T) {
	document, _, verified := concreteInputFixture(t, "ax-b01-correct")
	limits := concreteInputLimits(verified.Request)

	_, err := document.InspectProofSupports(
		verified.Request.AssurancePolicy,
		func(id protocol.Digest) ([]byte, error) {
			data, readErr := verified.ReadArtifact(id)
			return append(data, ' '), readErr
		},
		limits,
	)
	assertCompletenessCode(t, err, rejection.DigestMismatch)

	limits.MaxLogicalBytes = 1
	_, err = document.InspectProofSupports(
		verified.Request.AssurancePolicy, verified.ReadArtifact, limits,
	)
	assertCompletenessCode(t, err, rejection.ResourceLimit)

	limits = concreteInputLimits(verified.Request)
	limits.MaxSemanticSteps = 1
	_, err = document.InspectProofSupports(
		verified.Request.AssurancePolicy, verified.ReadArtifact, limits,
	)
	assertCompletenessCode(t, err, rejection.ResourceLimit)

	_, err = document.InspectProofSupports(
		verified.Request.AssurancePolicy, nil, concreteInputLimits(verified.Request),
	)
	assertCompletenessCode(t, err, rejection.ArtifactMissing)
}

func TestInspectProofSupportsIsDeterministic(t *testing.T) {
	document, _, verified := concreteInputFixture(t, "ax-b01-correct")
	want, err := document.InspectProofSupports(
		verified.Request.AssurancePolicy, verified.ReadArtifact, concreteInputLimits(verified.Request),
	)
	if err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 100; repeat++ {
		got, err := document.InspectProofSupports(
			verified.Request.AssurancePolicy, verified.ReadArtifact, concreteInputLimits(verified.Request),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("proof support inspection changed across identical runs")
		}
	}
}

type bundleReader struct {
	read ArtifactReader
}

func proofExecutionArtifacts(t *testing.T, document *Document) (protocol.Digest, protocol.Digest, protocol.Digest) {
	t.Helper()
	for _, execution := range document.executions {
		if execution.kind != "prove" || execution.result.kind != "completed" {
			continue
		}
		set, setOK := singleExecutionArtifact(execution.inputs, "obligation-set")
		query, queryOK := singleExecutionArtifact(execution.inputs, "query")
		response, responseOK := singleExecutionArtifact(execution.outputs, "response")
		if setOK && queryOK && responseOK {
			return set, query, response
		}
	}
	t.Fatal("completed proof execution not found")
	return protocol.Digest{}, protocol.Digest{}, protocol.Digest{}
}

func rebindProofArtifact(
	t *testing.T,
	document *Document,
	source bundleReader,
	role string,
	data []byte,
) ArtifactReader {
	t.Helper()
	set, query, response := proofExecutionArtifacts(t, document)
	old := map[string]protocol.Digest{
		"obligation-set": set,
		"query":          query,
		"response":       response,
	}[role]
	if old == (protocol.Digest{}) {
		t.Fatalf("unsupported proof artifact role %q", role)
	}
	newID := protocol.Digest(sha256.Sum256(data))
	definition := document.artifacts[old]
	document.artifacts[newID] = definition
	for executionID, execution := range document.executions {
		for index := range execution.inputs {
			if execution.inputs[index].role == role {
				execution.inputs[index].artifact = newID
			}
		}
		for index := range execution.outputs {
			if execution.outputs[index].role == role {
				execution.outputs[index].artifact = newID
			}
		}
		document.executions[executionID] = execution
	}
	for obligationID, result := range document.results {
		if result.kind != "proved" || result.support.kind != "backend-attestation" {
			continue
		}
		if role == "query" {
			result.support.query = newID
		}
		if role == "response" {
			result.support.response = newID
		}
		document.results[obligationID] = result
	}
	return func(id protocol.Digest) ([]byte, error) {
		if id == newID {
			return append([]byte(nil), data...), nil
		}
		return source.read(id)
	}
}
