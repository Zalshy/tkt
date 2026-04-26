package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func runManForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	manCmd.SetOut(&buf)
	defer manCmd.SetOut(nil)
	err := runMan(manCmd, args)
	return buf.String(), err
}

func TestMan_List(t *testing.T) {
	out, err := runManForTest(t)
	if err != nil {
		t.Fatalf("man list: %v", err)
	}
	if !strings.Contains(out, "minimal") || !strings.Contains(out, "state-machine") {
		t.Fatalf("expected key manual pages in list, got %q", out)
	}
}

func TestMan_ReadAndAlias(t *testing.T) {
	minimal, err := runManForTest(t, "minimal")
	if err != nil {
		t.Fatalf("man minimal: %v", err)
	}
	llm, err := runManForTest(t, "llm")
	if err != nil {
		t.Fatalf("man llm: %v", err)
	}
	if minimal != llm {
		t.Fatalf("llm alias did not match minimal")
	}
	if !strings.Contains(minimal, "Compact operating guide") {
		t.Fatalf("unexpected minimal page: %q", minimal)
	}
}

func TestMan_Missing(t *testing.T) {
	_, err := runManForTest(t, "missing")
	if err == nil || !strings.Contains(err.Error(), "Run: tkt man") {
		t.Fatalf("expected missing page hint, got %v", err)
	}
}

func TestManualHint(t *testing.T) {
	got := withManualHint("unknown command \"wat\" for \"tkt\"")
	if !strings.Contains(got, "tkt man minimal") {
		t.Fatalf("expected manual hint, got %q", got)
	}
}
