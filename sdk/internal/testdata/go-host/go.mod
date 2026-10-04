module example.com/vivy-go-host

go 1.26.4

require agent-vivy v0.0.0

// Only the embedder module is pinned locally on purpose: a plain consumer
// build must still fail because agent-vivy's own local replaces (laputa
// siblings, in-repo plugin modules, bml) are not inherited across module
// boundaries. The go-host pack target generates the full closure.
replace agent-vivy => ../../../..
