package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

type ReferenceKind string

const (
	ReferenceInput ReferenceKind = "input"
	ReferenceNode  ReferenceKind = "node"
)

type Reference struct {
	Kind   ReferenceKind
	Name   string
	NodeID string
}

type TemplatePart struct {
	Literal   string
	Reference *Reference
}

type Template struct {
	Original string
	Parts    []TemplatePart
}

var (
	inputNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)
	nodeIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
)

func ParseTemplate(source string) (Template, error) {
	template := Template{Original: source}
	for offset := 0; offset < len(source); {
		start := strings.Index(source[offset:], "${{")
		if start < 0 {
			literal := source[offset:]
			if strings.Contains(literal, "}}") {
				return Template{}, fmt.Errorf("template contains an unmatched closing delimiter")
			}
			if literal != "" {
				template.Parts = append(template.Parts, TemplatePart{Literal: literal})
			}
			break
		}
		start += offset
		if start > offset {
			literal := source[offset:start]
			if strings.Contains(literal, "}}") {
				return Template{}, fmt.Errorf("template contains an unmatched closing delimiter")
			}
			template.Parts = append(template.Parts, TemplatePart{Literal: literal})
		}
		end := strings.Index(source[start+3:], "}}")
		if end < 0 {
			return Template{}, fmt.Errorf("template reference is missing closing delimiter")
		}
		end += start + 3
		inner := strings.TrimSpace(source[start+3 : end])
		if inner == "" || strings.Contains(inner, "${{") || strings.Contains(inner, "}}") {
			return Template{}, fmt.Errorf("template reference is malformed")
		}
		ref, err := parseReference(inner)
		if err != nil {
			return Template{}, err
		}
		template.Parts = append(template.Parts, TemplatePart{Reference: &ref})
		offset = end + 2
	}
	return template, nil
}

func parseReference(path string) (Reference, error) {
	if strings.HasPrefix(path, "inputs.") {
		name := strings.TrimPrefix(path, "inputs.")
		if !inputNamePattern.MatchString(name) {
			return Reference{}, fmt.Errorf("template input reference %q is invalid", path)
		}
		return Reference{Kind: ReferenceInput, Name: name}, nil
	}
	if strings.HasPrefix(path, "nodes.") {
		parts := strings.Split(path, ".")
		if len(parts) != 3 || parts[2] != "output" || !nodeIDPattern.MatchString(parts[1]) {
			return Reference{}, fmt.Errorf("template node reference %q is invalid", path)
		}
		return Reference{Kind: ReferenceNode, NodeID: parts[1], Name: "output"}, nil
	}
	return Reference{}, fmt.Errorf("template reference %q is not allowed", path)
}

func (template Template) References() []Reference {
	refs := make([]Reference, 0)
	for _, part := range template.Parts {
		if part.Reference != nil {
			refs = append(refs, *part.Reference)
		}
	}
	return refs
}

func (template Template) String() string {
	if template.Original != "" {
		return template.Original
	}
	var out strings.Builder
	for _, part := range template.Parts {
		if part.Reference == nil {
			out.WriteString(part.Literal)
			continue
		}
		if part.Reference.Kind == ReferenceInput {
			fmt.Fprintf(&out, "${{ inputs.%s }}", part.Reference.Name)
		} else {
			fmt.Fprintf(&out, "${{ nodes.%s.output }}", part.Reference.NodeID)
		}
	}
	return out.String()
}

func (template Template) Resolve(inputs, outputs map[string]string) (string, error) {
	var out strings.Builder
	for _, part := range template.Parts {
		if part.Reference == nil {
			out.WriteString(part.Literal)
			continue
		}
		if part.Reference.Kind == ReferenceInput {
			value, ok := inputs[part.Reference.Name]
			if !ok {
				return "", fmt.Errorf("workflow input %q is missing", part.Reference.Name)
			}
			out.WriteString(value)
			continue
		}
		value, ok := outputs[part.Reference.NodeID]
		if !ok {
			return "", fmt.Errorf("workflow node output %q is missing", part.Reference.NodeID)
		}
		out.WriteString(value)
	}
	return out.String(), nil
}
