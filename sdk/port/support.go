package port

import (
	"fmt"

	"agent-vivy/sdk/module"
)

type SupportState string

const (
	SupportReserved           SupportState = "RESERVED"
	SupportSpecified          SupportState = "SPECIFIED"
	SupportCandidate          SupportState = "CANDIDATE"
	SupportSupported          SupportState = "SUPPORTED"
	SupportDeferredIndefinite SupportState = "DEFERRED-INDEFINITE"
)

type EvidenceKind string

const (
	EvidencePortDefinition    EvidenceKind = "port-definition"
	EvidenceSDKContract       EvidenceKind = "sdk-contract"
	EvidenceHostConsumer      EvidenceKind = "host-consumer"
	EvidenceRealProvider      EvidenceKind = "real-provider"
	EvidenceFailureModel      EvidenceKind = "failure-model"
	EvidenceConformanceSuite  EvidenceKind = "conformance-suite"
	EvidenceInspectProjection EvidenceKind = "inspect-projection"
)

var requiredEvidenceKinds = []EvidenceKind{
	EvidencePortDefinition,
	EvidenceSDKContract,
	EvidenceHostConsumer,
	EvidenceRealProvider,
	EvidenceFailureModel,
	EvidenceConformanceSuite,
	EvidenceInspectProjection,
}

type EvidenceReference struct {
	Kind EvidenceKind
	ID   string
}

type SupportEvidence struct {
	References  []EvidenceReference
	GatesPassed bool
}

func RequiredEvidenceKinds() []EvidenceKind {
	return append([]EvidenceKind(nil), requiredEvidenceKinds...)
}

func EvaluateSupport(_ Definition, evidence SupportEvidence) SupportState {
	seen := make(map[EvidenceKind]bool, len(evidence.References))
	for _, reference := range evidence.References {
		if reference.ID != "" {
			seen[reference.Kind] = true
		}
	}
	for _, required := range requiredEvidenceKinds {
		if !seen[required] {
			return SupportSpecified
		}
	}
	if evidence.GatesPassed {
		return SupportSupported
	}
	return SupportCandidate
}

func (catalog Catalog) RequireSelectable(ref module.PortRef, evidence SupportEvidence) error {
	definition, ok := catalog.Lookup(ref)
	if !ok {
		return fmt.Errorf("unknown Port %s", ref.Port)
	}
	state := EvaluateSupport(definition, evidence)
	if state != SupportSupported {
		return fmt.Errorf("Port %s is %s and cannot be selected", ref.Port, state)
	}
	return nil
}
