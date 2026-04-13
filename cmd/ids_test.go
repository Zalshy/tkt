package cmd

import (
	"strings"
	"testing"
)

func TestParseIDs_Single(t *testing.T) {
	ids, err := parseIDs("33")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "33" {
		t.Errorf("expected [33], got %v", ids)
	}
}

func TestParseIDs_Multiple(t *testing.T) {
	ids, err := parseIDs("33,34,35")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 || ids[0] != "33" || ids[1] != "34" || ids[2] != "35" {
		t.Errorf("expected [33 34 35], got %v", ids)
	}
}

func TestParseIDs_WhitespaceTrimmed(t *testing.T) {
	ids, err := parseIDs(" 33 , 34 , 35 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 || ids[0] != "33" || ids[1] != "34" || ids[2] != "35" {
		t.Errorf("expected [33 34 35], got %v", ids)
	}
}

func TestParseIDs_DuplicatesSilentlyDropped(t *testing.T) {
	ids, err := parseIDs("33,33,35")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "33" || ids[1] != "35" {
		t.Errorf("expected [33 35], got %v", ids)
	}
}

func TestParseIDs_EmptySegmentError(t *testing.T) {
	_, err := parseIDs("33,,34")
	if err == nil {
		t.Fatal("expected error for empty segment, got nil")
	}
	if !strings.Contains(err.Error(), "empty segment") {
		t.Errorf("expected 'empty segment' in error, got: %v", err)
	}
}

func TestParseIDs_EmptyStringError(t *testing.T) {
	_, err := parseIDs("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
	if !strings.Contains(err.Error(), "empty segment") {
		t.Errorf("expected 'empty segment' in error, got: %v", err)
	}
}
