package main

import (
	"strings"
	"testing"
)

func TestPrintModelsMarksConfigured(t *testing.T) {
	var out, errOut strings.Builder
	printModels(&out, &errOut, []string{"gemini-a", "gemini-4-neon", "gemini-z"}, "gemini-4-neon")
	if want := "  gemini-a\n* gemini-4-neon\n  gemini-z\n"; out.String() != want {
		t.Fatalf("output %q, want %q", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected warning %q", errOut.String())
	}
}

func TestPrintModelsWarnsWhenConfiguredIsMissing(t *testing.T) {
	var out, errOut strings.Builder
	printModels(&out, &errOut, []string{"gemini-a"}, "gemini-gone")
	if out.String() != "  gemini-a\n" {
		t.Fatalf("output %q", out.String())
	}
	if !strings.Contains(errOut.String(), "gemini-gone is not in this list") {
		t.Fatalf("warning %q", errOut.String())
	}
}
