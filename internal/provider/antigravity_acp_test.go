package provider

import "testing"

func TestACPResponsePartsText(t *testing.T) {
	parts, err := acpResponseParts(`{"text":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].Text != "hello" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestACPResponsePartsToolUse(t *testing.T) {
	parts, err := acpResponseParts(`{"tool_use":{"name":"Read","input":{"file_path":"/tmp/a"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("unexpected parts: %#v", parts)
	}
	if parts[0].FunctionCall.Name != "Read" || parts[0].FunctionCall.Args["file_path"] != "/tmp/a" {
		t.Fatalf("unexpected function call: %#v", parts[0].FunctionCall)
	}
}
