package app

import (
	"path/filepath"
	"testing"

	"agent-vivy/internal/config"
	"github.com/ProjectViVy/laputa/laputa/persona"
)

// Existing conversation fixtures represent an onboarded owner. Fresh-install
// and RPC initialization behavior is tested separately by the headless smoke.
func initializeTestPersona(t *testing.T, cfg config.Config) {
	t.Helper()
	authority, err := persona.Open(filepath.Join(cfg.DataDirectory(), "garden", "persona"))
	if err != nil {
		t.Fatal(err)
	}
	status, err := authority.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != persona.StatusUninitialized {
		return
	}
	_, err = authority.Initialize(persona.Initialization{
		Identity: "Vivy test assistant", Relationship: "Test partner",
		Redline: "Test boundaries", User: "Test preferences", World: "Test workspace",
	}, "test-owner", persona.SourceInit, "Conversation fixture setup")
	if err != nil {
		t.Fatal(err)
	}
}
