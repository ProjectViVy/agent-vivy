package rpc

import (
	"sort"

	"agent-vivy/internal/rpccontract"
)

type contributionDispatch struct {
	handlers     map[string]rpccontract.HandlerFunc
	capabilities []string
}

func newContributionDispatch(contributions []rpccontract.Contribution) (contributionDispatch, error) {
	bindings := make([]rpccontract.MethodBinding, 0)
	for _, contribution := range contributions {
		if contribution == nil {
			continue
		}
		bindings = append(bindings, contribution.RPCBindings()...)
	}
	if err := rpccontract.ValidateMethodBindings(coreMethodNames, bindings); err != nil {
		return contributionDispatch{}, err
	}

	handlers := make(map[string]rpccontract.HandlerFunc, len(bindings))
	capabilitySet := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		handlers[binding.Method] = binding.Handler
		capabilitySet[binding.Capability] = struct{}{}
	}
	capabilities := make([]string, 0, len(capabilitySet))
	for capability := range capabilitySet {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return contributionDispatch{handlers: handlers, capabilities: capabilities}, nil
}

var coreMethodNames = map[string]struct{}{
	"initialize": {}, "capabilities": {},
	"session/create": {}, "session/set_permission": {}, "session/set_workspace": {},
	"session/list": {}, "session/get": {}, "session/rename": {}, "session/delete": {},
	"session/messages": {}, "session/context": {}, "session/sidebar": {},
	"attachments/resolve": {}, "attachment/resolve": {},
	"project-context/resolve": {}, "project-context/list": {}, "context/compact": {},
	"session/rewind": {}, "session/fork": {}, "session/edit": {}, "session/todos": {},
	"session/todo/update": {}, "session/compactions": {}, "trajectory/session": {},
	"cron/list": {}, "cron/create": {}, "cron/update": {}, "cron/delete": {},
	"cron/trigger": {}, "cron/stop": {}, "turn/start": {}, "shell/start": {},
	"turn/interrupt": {}, "run/cancel": {}, "run/get": {}, "run/subscribe": {},
	"run/unsubscribe": {}, "run/log": {}, "workspace/list": {}, "workspace/read": {},
	"workspace/browse": {}, "approval/list": {}, "approval/respond": {},
	"question/list": {}, "question/respond": {}, "review/list": {}, "review/get": {},
	"review/respond": {}, "background/recover": {}, "background/list": {},
	"background/attach": {}, "child/start": {}, "child/get": {}, "child/list": {},
	"child/wait": {}, "child/cancel": {}, "generations/list": {}, "generations/get": {},
	"generations/create": {}, "generations/reject": {}, "evals/list": {}, "evals/record": {},
	"evals/start": {}, "promotions/list": {}, "promotions/promote": {}, "species/inspect": {},
	"settings/get": {}, "settings/locale": {}, "settings/update": {}, "settings/providers": {},
	"settings/providers/upsert": {}, "settings/providers/delete": {}, "settings/providers/refresh": {},
	"settings/model/select": {}, "settings/mcp": {}, "settings/mcp/upsert": {},
	"settings/mcp/delete": {}, "settings/mcp/probe": {}, "settings/mcp/resources": {},
	"settings/mcp/resources/list": {}, "mcp/resources": {}, "mcp/resources/list": {},
	"settings/mcp/read": {}, "settings/mcp/resources/read": {}, "mcp/read": {},
	"mcp/resources/read": {}, "tools/list": {}, "tools/set-active": {},
	"stats/tokens": {},
	"skills/list":  {}, "skills/get": {}, "skills/set-enabled": {}, "commands/list": {},
	"commands/expand": {}, "skills/revisions/list": {}, "skills/marketplace/search": {},
	"skills/marketplace/featured": {}, "skills/marketplace/install": {},
	"skills/marketplace/check": {}, ModuleActionMethod: {},
}
