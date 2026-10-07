package a2aserver

import (
	"context"
	"strings"

	"agent-vivy/sdk/port/channel"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// buildCard projects the Host's safe TaskServiceInfo plus the module's
// public_* settings onto an AgentCard. Only configured, compiled
// capabilities are advertised: bearer auth, streaming and input
// continuation when the Host reports them, no extensions.
func buildCard(info channel.TaskServiceInfo, s a2aSettings, endpoint string) *a2a.AgentCard {
	name := s.PublicName
	if name == "" {
		name = info.Name
	}
	desc := s.PublicDescription
	if desc == "" {
		desc = info.Description
	}
	// public_skill_ids filters the host-reported skills to the advertised
	// subset; an empty filter advertises every declared skill.
	wanted := map[string]bool{}
	for _, id := range s.PublicSkillIDs {
		wanted[id] = true
	}
	var skills []a2a.AgentSkill
	for _, sk := range info.Skills {
		if len(wanted) > 0 && !wanted[sk.ID] {
			continue
		}
		skills = append(skills, a2a.AgentSkill{
			ID: sk.ID, Name: sk.Name, Description: sk.Description,
			Tags: sk.Tags, Examples: sk.Examples,
			InputModes: []string{"text/plain"}, OutputModes: []string{"text/plain"},
		})
	}
	return &a2a.AgentCard{
		Name:               name,
		Description:        desc,
		Version:            info.Version,
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Capabilities: a2a.AgentCapabilities{
			Streaming: info.Streaming,
			// Input continuation = tasks/… sends on an existing task ID.
			PushNotifications: false,
			ExtendedAgentCard: false,
		},
		SupportedInterfaces: []*a2a.AgentInterface{{
			URL:             strings.TrimRight(endpoint, "/") + "/a2a",
			ProtocolBinding: a2a.TransportProtocolJSONRPC,
			ProtocolVersion: a2a.Version,
		}},
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"bearer": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer"},
		},
		SecurityRequirements: a2a.SecurityRequirementsOptions{
			{"bearer": a2a.SecuritySchemeScopes{}},
		},
		Skills: skills,
	}
}

// card resolves the advertised card from Host service info + settings.
// The endpoint is the request's own origin — a forged Host header never
// rewrites it because the listener pins the configured public_base_url.
func (h *requestHandler) card(ctx context.Context) (*a2a.AgentCard, error) {
	info, err := h.info.TaskServiceInfo(ctx)
	if err != nil {
		return nil, err
	}
	return buildCard(info, h.settings, info.PublicEndpoint), nil
}
