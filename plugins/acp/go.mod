module agent-vivy/plugins/acp

go 1.26.4

require (
	agent-vivy v0.0.0
	github.com/eino-contrib/acp v0.0.4
)

// ACP-01 ruling (route A): develop against the owner fork carrying the
// accepted connection options + sanitized wire errors until eino-contrib/acp
// merges the upstream PR; revert this replace then.
replace github.com/eino-contrib/acp => github.com/mastwet/acp v0.0.0-20261007071707-780fb4dd4b98

replace agent-vivy => ../..
