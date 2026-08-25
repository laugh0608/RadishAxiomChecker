package axiomevidence

import (
	"crypto/sha256"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// ConcreteInputLimits binds JSON parsing, deterministic logical-memory
// accounting, and assume evaluation to explicit checker request budgets.
type ConcreteInputLimits struct {
	JSON             strictjson.Limits
	MaxLogicalBytes  uint64
	MaxSemanticSteps uint64
}

// ConcreteInputCheck identifies the concrete input artifacts independently
// classified during this verification call.
type ConcreteInputCheck struct {
	Artifacts []protocol.Digest
	Checked   int
	Failed    int
}

// ArtifactReader returns raw bytes for one content digest. A bundle-backed
// caller should use bundle.Verified.ReadArtifact so identity is rechecked at
// read time.
type ArtifactReader func(protocol.Digest) ([]byte, error)

// VerifyConcreteInputs binds each artifact-level input-conformance obligation
// to its host-input execution artifact, independently decodes the complete
// axiom-benchmark-data input, checks IR WF, evaluates assume/Pre, and checks
// the declared checked/failed classification. It does not execute transform
// nodes, compare outputs, or confirm any non-input counterexample target.
func (document Document) VerifyConcreteInputs(
	ir axiomir.Document,
	read ArtifactReader,
	limits ConcreteInputLimits,
) (ConcreteInputCheck, error) {
	if err := document.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest); err != nil {
		return ConcreteInputCheck{}, err
	}
	if read == nil {
		return ConcreteInputCheck{}, rejection.New(rejection.ArtifactMissing, "concrete input artifact reader is unavailable")
	}

	results := make(map[protocol.Digest]obligationResult)
	for id, definition := range document.obligations {
		if definition.Kind != "input-conformance" || definition.Subject.Kind != "artifact" {
			continue
		}
		artifact := definition.Subject.Artifact
		if _, duplicate := results[artifact]; duplicate {
			return ConcreteInputCheck{}, concreteMismatch("multiple input-conformance obligations target one artifact")
		}
		result, exists := document.results[id]
		if !exists {
			return ConcreteInputCheck{}, concreteMismatch("input-conformance obligation result is unavailable")
		}
		results[artifact] = result
	}

	hostInputs := make(map[protocol.Digest]struct{})
	for _, execution := range document.executions {
		for _, input := range execution.inputs {
			if input.role == "host-input" {
				hostInputs[input.artifact] = struct{}{}
			}
		}
	}
	for artifact := range hostInputs {
		if _, exists := results[artifact]; !exists {
			return ConcreteInputCheck{}, concreteMismatch("host-input execution artifact has no input-conformance obligation")
		}
	}
	for artifact := range results {
		if _, exists := hostInputs[artifact]; !exists {
			return ConcreteInputCheck{}, concreteMismatch("input-conformance artifact is not bound as host-input")
		}
	}

	artifacts := make([]protocol.Digest, 0, len(results))
	for artifact := range results {
		artifacts = append(artifacts, artifact)
	}
	sortDigests(artifacts)
	summary := ConcreteInputCheck{Artifacts: append([]protocol.Digest(nil), artifacts...)}
	for _, artifact := range artifacts {
		definition, exists := document.artifacts[artifact]
		if !exists || definition.format != "axiom-benchmark-data" || definition.formatVersion != "0.1" {
			return ConcreteInputCheck{}, concreteMismatch("host-input artifact is not declared as axiom-benchmark-data 0.1")
		}
		data, err := read(artifact)
		if err != nil {
			return ConcreteInputCheck{}, err
		}
		if protocol.Digest(sha256.Sum256(data)) != artifact {
			return ConcreteInputCheck{}, rejection.New(rejection.DigestMismatch, "concrete input bytes do not match their artifact digest")
		}
		input, err := ir.DecodeBenchmarkInput(data, limits.JSON)
		if err != nil {
			return ConcreteInputCheck{}, err
		}
		if limits.MaxLogicalBytes == 0 || input.LogicalBytes() > limits.MaxLogicalBytes {
			return ConcreteInputCheck{}, rejection.New(rejection.ResourceLimit, "concrete input exceeds its logical-memory limit")
		}
		worldCheck := ir.CheckCompleteInputWorld(input.World)
		preSatisfied := false
		if worldCheck.WellFormed {
			evaluation, err := ir.EvaluateAssumes(input.World, limits.MaxSemanticSteps)
			if err != nil {
				return ConcreteInputCheck{}, err
			}
			preSatisfied = evaluation.AllTrue
		}

		result := results[artifact]
		switch result.kind {
		case "checked":
			if input.Role != "input" || !worldCheck.WellFormed || !preSatisfied {
				return ConcreteInputCheck{}, concreteMismatch("checked input-conformance is not supported by complete WF and Pre")
			}
			summary.Checked++
		case "failed":
			if input.Role != "invalid-input" || worldCheck.WellFormed && preSatisfied {
				return ConcreteInputCheck{}, concreteMismatch("failed input-conformance is not supported by a WF or Pre failure")
			}
			if result.counterexample == nil || len(result.counterexample.worlds) == 0 {
				return ConcreteInputCheck{}, concreteMismatch("failed input-conformance has no retained world projection")
			}
			for _, projection := range result.counterexample.worlds {
				if !axiomir.WorldContainsProjection(input.World, projection) {
					return ConcreteInputCheck{}, concreteMismatch("input counterexample world is not a projection of its bound artifact")
				}
			}
			summary.Failed++
		default:
			return ConcreteInputCheck{}, concreteMismatch("input-conformance result is neither checked nor failed")
		}
	}
	return summary, nil
}

func concreteMismatch(detail string) error {
	return rejection.New(rejection.ConcreteCheckMismatch, detail)
}
