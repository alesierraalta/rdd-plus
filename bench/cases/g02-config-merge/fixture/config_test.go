package config

import "testing"

func TestOverrideWins(t *testing.T) {
	got := Merge(Defaults, Config{Port: 9090, Region: "eu-west-1"})
	if got.Port != 9090 || got.Region != "eu-west-1" {
		t.Fatalf("override not applied: %+v", got)
	}
}

func TestDebugCanBeEnabled(t *testing.T) {
	got := Merge(Defaults, Config{Debug: boolp(true)})
	if got.Debug == nil || !*got.Debug {
		t.Fatal("debug should be enabled by the override")
	}
}

func TestRetriesOverride(t *testing.T) {
	got := Merge(Defaults, Config{Retries: intp(5)})
	if got.Retries == nil || *got.Retries != 5 {
		t.Fatalf("retries = %v, want 5", got.Retries)
	}
}

func TestLayersApplyInOrder(t *testing.T) {
	got := Load(Config{Port: 9000, Region: "eu-west-1"}, Config{Port: 9100})
	if got.Port != 9100 || got.Region != "eu-west-1" || got.Retries == nil || *got.Retries != 3 {
		t.Fatalf("layers misapplied: %+v", got)
	}
}
