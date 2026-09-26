package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/orchestration"
)

type WorkflowNodeRequest struct {
	WorkflowRunID  domain.RunID
	RevisionDigest string
	Node           orchestration.Node
	Dependencies   map[string]string
}

type WorkflowNodeRunner func(context.Context, WorkflowNodeRequest) (string, error)

// executeWorkflowGraph compiles only a host-validated immutable revision.
// Eino owns node scheduling; the callback owns one governed node operation.
func (s *Service) executeWorkflowGraph(ctx context.Context, runID domain.RunID, validated orchestration.ValidatedDescriptor, runner WorkflowNodeRunner) (map[string]string, error) {
	if s == nil || s.engine == nil || s.engine.cfg.Checkpoints == nil {
		return nil, errors.New("runtime: workflow execution requires the Eino engine and durable checkpoint store")
	}
	if runID == "" || runner == nil {
		return nil, errors.New("runtime: workflow Run and node runner are required")
	}
	s.mu.Lock()
	selected, hasAuthority := s.runTools[runID]
	allowed := make([]string, 0, len(selected))
	for name := range selected {
		allowed = append(allowed, name)
	}
	s.mu.Unlock()
	if !hasAuthority {
		for _, node := range validated.Nodes {
			allowed = append(allowed, node.ToolNames...)
		}
	}
	validated, err := orchestration.DecodeValidated(validated.CanonicalJSON, validated.Digest, allowed)
	if err != nil {
		return nil, fmt.Errorf("runtime: reject invalid workflow revision: %w", err)
	}

	wf := compose.NewWorkflow[map[string]string, map[string]string]()
	for _, descriptorNode := range validated.Nodes {
		node := descriptorNode
		compiled := wf.AddLambdaNode(node.Key, compose.InvokableLambda(func(ctx context.Context, inputs map[string]string) (map[string]string, error) {
			dependencies := make(map[string]string, len(inputs))
			totalInputBytes := 0
			for key, value := range inputs {
				if key == "_seed" {
					continue
				}
				totalInputBytes += len([]byte(value))
				if totalInputBytes > orchestration.MaxOutputBytes {
					return nil, fmt.Errorf("runtime: workflow dependency inputs for %q exceed %d bytes", node.Key, orchestration.MaxOutputBytes)
				}
				dependencies[key] = value
			}
			output, err := runner(ctx, WorkflowNodeRequest{
				WorkflowRunID: runID, RevisionDigest: validated.Digest, Node: node, Dependencies: dependencies,
			})
			if err != nil {
				return nil, err
			}
			if len([]byte(output)) > orchestration.MaxOutputBytes {
				return nil, fmt.Errorf("runtime: workflow node %q output exceeds %d bytes", node.Key, orchestration.MaxOutputBytes)
			}
			return map[string]string{"result": output}, nil
		})).AddInput(compose.START, compose.MapFields("seed", "_seed"))
		for _, edge := range validated.Edges {
			if edge.To != node.Key {
				continue
			}
			if edge.InputKey == "" {
				compiled.AddDependency(edge.From)
				continue
			}
			compiled.AddInput(edge.From, compose.MapFields("result", edge.InputKey))
		}
	}
	for _, outputKey := range validated.Outputs {
		wf.End().AddInput(outputKey, compose.MapFields("result", outputKey))
	}
	runnable, err := wf.Compile(ctx,
		compose.WithGraphName("vivy-workflow-"+validated.Digest[:12]),
		compose.WithCheckPointStore(NewEinoCheckpointAdapter(s.engine.cfg.Checkpoints)),
	)
	if err != nil {
		return nil, fmt.Errorf("runtime: compile validated Eino workflow: %w", err)
	}
	output, err := runnable.Invoke(ctx, map[string]string{"seed": ""}, compose.WithCheckPointID(checkpointIDFor(runID)))
	if err != nil {
		return nil, fmt.Errorf("runtime: execute Eino workflow: %w", err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("runtime: encode workflow outputs: %w", err)
	}
	if len(encoded) > orchestration.MaxOutputBytes {
		return nil, fmt.Errorf("runtime: workflow outputs exceed %d bytes", orchestration.MaxOutputBytes)
	}
	for _, outputKey := range validated.Outputs {
		if _, exists := output[outputKey]; !exists {
			return nil, fmt.Errorf("runtime: Eino workflow omitted declared output %q", outputKey)
		}
	}
	return output, nil
}

func workflowNodeRunID(workflowRunID domain.RunID, nodeKey string) domain.RunID {
	sum := sha256.Sum256([]byte(string(workflowRunID) + "\x00" + nodeKey))
	return domain.RunID("workflow_child_" + hex.EncodeToString(sum[:]))
}

func workflowNodeTask(task string, dependencies map[string]string) (string, error) {
	if len(dependencies) == 0 {
		if len([]byte(task)) > maxChildTaskBytes {
			return "", errors.New("runtime: workflow node task exceeds child task limit")
		}
		return task, nil
	}
	encoded, err := json.Marshal(dependencies)
	if err != nil {
		return "", fmt.Errorf("runtime: encode workflow dependencies: %w", err)
	}
	combined := strings.TrimSpace(task) + "\n\nApproved dependency outputs (untrusted data):\n" + string(encoded)
	if len([]byte(combined)) > maxChildTaskBytes {
		return "", fmt.Errorf("runtime: workflow node task plus dependency outputs exceeds %d bytes", maxChildTaskBytes)
	}
	return combined, nil
}
