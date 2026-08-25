package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

func (p *parser) parseArtifacts(value strictjson.Value) (int, error) {
	items, err := array(value)
	if err != nil {
		return 0, err
	}
	var previous string
	for index, item := range items {
		fields, err := object(item, "byte_length", "content_digest", "format", "format_version")
		if err != nil {
			return 0, err
		}
		if _, err := canonicalUnsigned(fields["byte_length"]); err != nil {
			return 0, err
		}
		spelling, err := text(fields["content_digest"])
		if err != nil {
			return 0, err
		}
		if err := requireStrictOrder(previous, spelling, index != 0, "Axiom Evidence artifacts are not sorted and unique by content digest"); err != nil {
			return 0, err
		}
		id, err := protocol.ParseDigest(spelling)
		if err != nil {
			return 0, err
		}
		format, err := nonemptyText(fields["format"])
		if err != nil {
			return 0, err
		}
		version, err := nonemptyText(fields["format_version"])
		if err != nil {
			return 0, err
		}
		p.artifacts[id] = artifactDefinition{format: format, formatVersion: version}
		previous = spelling
	}
	return len(items), nil
}

var toolRoles = map[string]struct{}{
	"certificate-checker":     {},
	"counterexample-replayer": {},
	"evidence-producer":       {},
	"fixture-checker":         {},
	"host-executor":           {},
	"ir-normalizer":           {},
	"obligation-generator":    {},
	"output-comparator":       {},
	"prover":                  {},
}

func (p *parser) parseTools(value strictjson.Value) (int, error) {
	return parseDefinitionEntries(value, domainTool, p.tools, func(definition strictjson.Value, id protocol.Digest) error {
		fields, err := object(definition, "artifact", "name", "roles", "version")
		if err != nil {
			return err
		}
		artifact, err := digest(fields["artifact"])
		if err != nil {
			return err
		}
		p.artifactRefs[artifact] = struct{}{}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return err
		}
		roles, err := parseStringSet(fields["roles"], true, toolRoles, "Axiom Evidence tool roles are empty, unknown, unsorted, or duplicate")
		if err != nil {
			return err
		}
		version, err := nonemptyText(fields["version"])
		if err != nil {
			return err
		}
		if version == "latest" {
			return rejection.New(rejection.InvalidJSON, "Axiom Evidence tool version must be immutable")
		}
		roleSet := make(map[string]struct{}, len(roles))
		for _, role := range roles {
			roleSet[role] = struct{}{}
		}
		p.tools[id] = toolDefinition{
			artifact: artifact,
			name:     name,
			roles:    roleSet,
			version:  version,
		}
		return nil
	})
}

var executionKinds = map[string]struct{}{
	"check-certificate":     {},
	"check-fixture":         {},
	"compare-output":        {},
	"execute-host":          {},
	"generate-obligations":  {},
	"normalize":             {},
	"prove":                 {},
	"replay-counterexample": {},
}

func (p *parser) parseExecutions(value strictjson.Value) (int, error) {
	return parseDefinitionEntries(value, domainExecution, p.executions, func(definition strictjson.Value, id protocol.Digest) error {
		fields, err := object(definition, "inputs", "kind", "limits", "outputs", "result", "tool")
		if err != nil {
			return err
		}
		kind, err := requireOneOf(fields["kind"], executionKinds, "unsupported Axiom Evidence execution kind")
		if err != nil {
			return err
		}
		inputs, err := p.parseExecutionIO(fields["inputs"])
		if err != nil {
			return err
		}
		outputs, err := p.parseExecutionIO(fields["outputs"])
		if err != nil {
			return err
		}
		if err := parseExecutionLimits(fields["limits"]); err != nil {
			return err
		}
		result, err := parseExecutionResult(fields["result"])
		if err != nil {
			return err
		}
		tool, err := digest(fields["tool"])
		if err != nil {
			return err
		}
		p.toolRefs[tool] = struct{}{}
		p.executions[id] = executionDefinition{
			kind: kind, result: result, tool: tool, inputs: inputs, outputs: outputs,
		}
		return nil
	})
}

func (p *parser) parseExecutionIO(value strictjson.Value) ([]executionIO, error) {
	items, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]executionIO, 0, len(items))
	var previousRole string
	var previousArtifact string
	for index, item := range items {
		fields, err := object(item, "artifact", "role")
		if err != nil {
			return nil, err
		}
		role, err := nonemptyText(fields["role"])
		if err != nil {
			return nil, err
		}
		artifactSpelling, err := text(fields["artifact"])
		if err != nil {
			return nil, err
		}
		artifact, err := protocol.ParseDigest(artifactSpelling)
		if err != nil {
			return nil, err
		}
		if index != 0 && (previousRole > role || previousRole == role && previousArtifact >= artifactSpelling) {
			return nil, rejection.New(rejection.NoncanonicalOrder, "Axiom Evidence execution I/O is not sorted and unique by role and artifact")
		}
		previousRole = role
		previousArtifact = artifactSpelling
		p.artifactRefs[artifact] = struct{}{}
		result = append(result, executionIO{artifact: artifact, role: role})
	}
	return result, nil
}

func parseExecutionLimits(value strictjson.Value) error {
	items, err := array(value)
	if err != nil {
		return err
	}
	var previousName string
	var previousUnit string
	for index, item := range items {
		fields, err := object(item, "name", "unit", "value")
		if err != nil {
			return err
		}
		name, err := nonemptyText(fields["name"])
		if err != nil {
			return err
		}
		unit, err := nonemptyText(fields["unit"])
		if err != nil {
			return err
		}
		if _, err := canonicalUnsigned(fields["value"]); err != nil {
			return err
		}
		if index != 0 && (previousName > name || previousName == name && previousUnit >= unit) {
			return rejection.New(rejection.NoncanonicalOrder, "Axiom Evidence execution limits are not sorted and unique by name and unit")
		}
		previousName = name
		previousUnit = unit
	}
	return nil
}

func parseExecutionResult(value strictjson.Value) (executionResult, error) {
	tagValue, err := member(value, "kind")
	if err != nil {
		return executionResult{}, err
	}
	tag, err := text(tagValue)
	if err != nil {
		return executionResult{}, err
	}
	switch tag {
	case "completed":
		_, err = object(value, "kind")
		return executionResult{kind: tag}, err
	case "error", "resource-exhausted", "timeout", "unavailable", "unsupported":
		fields, objectErr := object(value, "code", "kind")
		if objectErr != nil {
			return executionResult{}, objectErr
		}
		code, err := nonemptyText(fields["code"])
		return executionResult{kind: tag, code: code}, err
	default:
		return executionResult{}, rejection.New(rejection.UnknownTag, "unsupported Axiom Evidence execution result kind")
	}
}

func parseDefinitionEntries[T any](
	value strictjson.Value,
	domain string,
	index map[protocol.Digest]T,
	parseDefinition func(strictjson.Value, protocol.Digest) error,
) (int, error) {
	items, err := array(value)
	if err != nil {
		return 0, err
	}
	var previous string
	for position, item := range items {
		fields, err := object(item, "definition", "id")
		if err != nil {
			return 0, err
		}
		spelling, err := text(fields["id"])
		if err != nil {
			return 0, err
		}
		if err := requireStrictOrder(previous, spelling, position != 0, "Axiom Evidence entries are not sorted and unique by ID"); err != nil {
			return 0, err
		}
		id, err := protocol.ParseDigest(spelling)
		if err != nil {
			return 0, err
		}
		if err := parseDefinition(fields["definition"], id); err != nil {
			return 0, err
		}
		if err := verifyDefinitionID(domain, fields["definition"], id); err != nil {
			return 0, err
		}
		if _, ok := index[id]; !ok {
			return 0, rejection.New(rejection.InvalidJSON, "Axiom Evidence parser did not index a definition")
		}
		previous = spelling
	}
	return len(items), nil
}
