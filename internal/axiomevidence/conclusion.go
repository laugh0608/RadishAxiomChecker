package axiomevidence

import (
	"sort"

	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

const (
	ConclusionImplementationInconsistent = "implementation_inconsistent"
	ConclusionInconclusive               = "inconclusive"
	ConclusionInputRejected              = "input_rejected"
	ConclusionSatisfied                  = "satisfied"
	ConclusionStructureRejected          = "structure_rejected"
	ConclusionViolated                   = "violated"

	ConclusionRefExecution  = "execution"
	ConclusionRefObligation = "obligation"
)

// ConclusionLimits binds deterministic ref construction and state traversal to
// explicit checker request budgets. It does not stand in for cross-stage
// cumulative accounting, which remains a later independent-result concern.
type ConclusionLimits struct {
	MaxLogicalBytes  uint64
	MaxSemanticSteps uint64
}

// ConclusionReference distinguishes obligation and execution domains even
// though both are represented by SHA-256 strings in producer Evidence.
type ConclusionReference struct {
	Kind  string
	Value protocol.Digest
}

// ConclusionCheck is the independently recomputed production conclusion. It is
// not an accepted/incomplete/rejected independent checker result.
type ConclusionCheck struct {
	Kind string
	Refs []ConclusionReference
}

type conclusionBudget struct {
	logicalBytes uint64
	steps        uint64
	limits       ConclusionLimits
}

// VerifyConclusion independently recomputes the producer Evidence conclusion
// from the already retained obligation states and required execution results,
// then compares kind and decisive refs exactly. Proof-support coverage and
// remaining trust stay separate and cannot be hidden by a satisfied producer
// conclusion.
func (document Document) VerifyConclusion(limits ConclusionLimits) (ConclusionCheck, error) {
	computed, err := recomputeConclusion(nil, document.obligations, document.results, document.executions, limits)
	if err != nil {
		return ConclusionCheck{}, err
	}
	if document.conclusion.kind != computed.Kind || len(document.conclusion.refs) != len(computed.Refs) {
		return ConclusionCheck{}, conclusionMismatch("producer Evidence conclusion kind or ref cardinality differs from independent recomputation")
	}
	for index, expected := range computed.Refs {
		actual := document.conclusion.refs[index]
		if actual.kind != expected.Kind || actual.value != expected.Value {
			return ConclusionCheck{}, conclusionMismatch("producer Evidence conclusion refs differ from independent recomputation")
		}
	}
	return computed, nil
}

// recomputeConclusion accepts a structure failure only from an earlier subject
// check. ParseStructure cannot manufacture a valid Document for a rejected
// subject, so VerifyConclusion deliberately passes nil for this branch.
func recomputeConclusion(
	structureFailure *conclusionReference,
	obligations map[protocol.Digest]axiomir.ObligationDefinition,
	results map[protocol.Digest]obligationResult,
	executions map[protocol.Digest]executionDefinition,
	limits ConclusionLimits,
) (ConclusionCheck, error) {
	budget := conclusionBudget{limits: limits}
	if err := budget.charge(0, 1); err != nil {
		return ConclusionCheck{}, err
	}
	if structureFailure != nil {
		ref, err := budget.reference(*structureFailure)
		if err != nil {
			return ConclusionCheck{}, err
		}
		return makeConclusion(ConclusionStructureRejected, []ConclusionReference{ref}), nil
	}

	ids := make([]protocol.Digest, 0, len(obligations))
	for id := range obligations {
		if err := budget.charge(32, 1); err != nil {
			return ConclusionCheck{}, err
		}
		ids = append(ids, id)
	}
	sortDigests(ids)

	inputFailures := make([]ConclusionReference, 0)
	hostFailures := make([]ConclusionReference, 0)
	otherFailures := make([]ConclusionReference, 0)
	blockers := make([]ConclusionReference, 0)
	coreSatisfied := true

	for _, id := range ids {
		definition := obligations[id]
		result, ok := results[id]
		if !ok {
			ref, err := budget.reference(conclusionReference{kind: ConclusionRefObligation, value: id})
			if err != nil {
				return ConclusionCheck{}, err
			}
			blockers = append(blockers, ref)
			if !hostOrOutput(definition) {
				coreSatisfied = false
			}
			continue
		}

		if result.kind == "failed" {
			ref, err := budget.reference(conclusionReference{kind: ConclusionRefObligation, value: id})
			if err != nil {
				return ConclusionCheck{}, err
			}
			switch {
			case definition.Kind == "input-conformance":
				inputFailures = append(inputFailures, ref)
				coreSatisfied = false
			case hostOrOutput(definition):
				hostFailures = append(hostFailures, ref)
			default:
				otherFailures = append(otherFailures, ref)
				coreSatisfied = false
			}
			continue
		}

		if !expectedConclusionState(definition.Expectation, result.kind) {
			ref, err := budget.reference(conclusionReference{kind: ConclusionRefObligation, value: id})
			if err != nil {
				return ConclusionCheck{}, err
			}
			blockers = append(blockers, ref)
			if !hostOrOutput(definition) {
				coreSatisfied = false
			}
			continue
		}

		executionID, required := requiredConclusionExecution(result)
		if required {
			if err := budget.charge(0, 1); err != nil {
				return ConclusionCheck{}, err
			}
			execution, exists := executions[executionID]
			if !exists {
				return ConclusionCheck{}, conclusionMismatch("required conclusion execution is unavailable")
			}
			if execution.result.kind != "completed" {
				ref, err := budget.reference(conclusionReference{kind: ConclusionRefExecution, value: executionID})
				if err != nil {
					return ConclusionCheck{}, err
				}
				blockers = append(blockers, ref)
				if !hostOrOutput(definition) {
					coreSatisfied = false
				}
			}
		}
	}

	switch {
	case len(inputFailures) != 0:
		return makeConclusion(ConclusionInputRejected, inputFailures), nil
	case len(hostFailures) != 0 && len(otherFailures) == 0 && coreSatisfied:
		return makeConclusion(ConclusionImplementationInconsistent, hostFailures), nil
	case len(hostFailures)+len(otherFailures) != 0:
		return makeConclusion(ConclusionViolated, append(hostFailures, otherFailures...)), nil
	case len(blockers) != 0:
		return makeConclusion(ConclusionInconclusive, blockers), nil
	default:
		return ConclusionCheck{Kind: ConclusionSatisfied, Refs: []ConclusionReference{}}, nil
	}
}

func expectedConclusionState(expectation, state string) bool {
	switch expectation {
	case "prove":
		return state == "proved"
	case "check":
		return state == "checked"
	case "trust":
		return state == "trusted"
	default:
		return false
	}
}

func requiredConclusionExecution(result obligationResult) (protocol.Digest, bool) {
	switch result.kind {
	case "proved":
		return result.support.execution, true
	case "checked":
		return result.execution, true
	default:
		return protocol.Digest{}, false
	}
}

func hostOrOutput(definition axiomir.ObligationDefinition) bool {
	return definition.Kind == "host-conformance" || definition.Kind == "output-conformance"
}

func makeConclusion(kind string, refs []ConclusionReference) ConclusionCheck {
	ordered := append([]ConclusionReference(nil), refs...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].Kind != ordered[right].Kind {
			return ordered[left].Kind < ordered[right].Kind
		}
		return ordered[left].Value.String() < ordered[right].Value.String()
	})
	deduplicated := ordered[:0]
	for _, ref := range ordered {
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != ref {
			deduplicated = append(deduplicated, ref)
		}
	}
	return ConclusionCheck{Kind: kind, Refs: deduplicated}
}

func (budget *conclusionBudget) reference(ref conclusionReference) (ConclusionReference, error) {
	if err := budget.charge(32, 1); err != nil {
		return ConclusionReference{}, err
	}
	return ConclusionReference{Kind: ref.kind, Value: ref.value}, nil
}

func (budget *conclusionBudget) charge(logicalBytes, steps uint64) error {
	if budget.limits.MaxLogicalBytes == 0 || logicalBytes > budget.limits.MaxLogicalBytes-budget.logicalBytes {
		return rejection.New(rejection.ResourceLimit, "conclusion recomputation exceeds its logical-memory limit")
	}
	if budget.limits.MaxSemanticSteps == 0 || steps > budget.limits.MaxSemanticSteps-budget.steps {
		return rejection.New(rejection.ResourceLimit, "conclusion recomputation exceeds its semantic-step limit")
	}
	budget.logicalBytes += logicalBytes
	budget.steps += steps
	return nil
}

func conclusionMismatch(detail string) error {
	return rejection.New(rejection.ConclusionMismatch, detail)
}
