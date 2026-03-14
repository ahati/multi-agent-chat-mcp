// Package common_test provides tests for common utilities
package common

import (
	"testing"
	"time"
)

// TestGenerateID_Unique verifies generated IDs are unique
func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := GenerateID(PrefixAgent)
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

// TestGenerateID_Format verifies ID format
func TestGenerateID_Format(t *testing.T) {
	id := GenerateID(PrefixAgent)
	if len(id) == 0 {
		t.Error("Generated ID is empty")
	}

	// Should contain prefix
	if id[:6] != "agent-" {
		t.Errorf("ID should start with 'agent-', got: %s", id)
	}
}

// TestGenerateAgentID verifies agent ID generation
func TestGenerateAgentID(t *testing.T) {
	id := GenerateAgentID()
	if id[:6] != "agent-" {
		t.Errorf("Agent ID should start with 'agent-', got: %s", id)
	}
}

// TestGenerateMessageID verifies message ID generation
func TestGenerateMessageID(t *testing.T) {
	id := GenerateMessageID()
	if id[:4] != "msg-" {
		t.Errorf("Message ID should start with 'msg-', got: %s", id)
	}
}

// TestGenerateTaskID verifies task ID generation
func TestGenerateTaskID(t *testing.T) {
	id := GenerateTaskID()
	if id[:5] != "task-" {
		t.Errorf("Task ID should start with 'task-', got: %s", id)
	}
}

// TestGenerateTodoID verifies todo ID generation
func TestGenerateTodoID(t *testing.T) {
	id := GenerateTodoID()
	if id[:5] != "todo-" {
		t.Errorf("Todo ID should start with 'todo-', got: %s", id)
	}
}

// TestValidateAgentName_Valid verifies valid names pass
func TestValidateAgentName_Valid(t *testing.T) {
	validNames := []string{
		"agent1",
		"my-agent",
		"Agent_123",
		"a",
		"agent-with-a-very-long-name-that-is-exactly-64-characters-long",
	}

	for _, name := range validNames {
		err := ValidateAgentName(name)
		if err != nil {
			t.Errorf("Expected name '%s' to be valid, got error: %v", name, err)
		}
	}
}

// TestValidateAgentName_Invalid verifies invalid names fail
func TestValidateAgentName_Invalid(t *testing.T) {
	invalidNames := []struct {
		name string
		err  error
	}{
		{"", ErrEmptyName},
		{"this-is-a-very-long-name-that-exceeds-sixty-four-characters-longX", ErrNameTooLong}, // 65 chars
		{"agent name", ErrInvalidName}, // space
		{"agent@name", ErrInvalidName}, // special char
	}

	for _, tc := range invalidNames {
		err := ValidateAgentName(tc.name)
		if err != tc.err {
			t.Errorf("Expected error %v for name '%s' (len=%d), got: %v", tc.err, tc.name, len(tc.name), err)
		}
	}
}

// TestValidateTaskID_Valid verifies valid task IDs pass
func TestValidateTaskID_Valid(t *testing.T) {
	err := ValidateTaskID("task-123")
	if err != nil {
		t.Errorf("Expected task ID to be valid, got: %v", err)
	}
}

// TestValidateTaskID_Invalid verifies invalid task IDs fail
func TestValidateTaskID_Invalid(t *testing.T) {
	err := ValidateTaskID("")
	if err != ErrEmptyTaskID {
		t.Errorf("Expected ErrEmptyTaskID, got: %v", err)
	}
}

// TestMarshalJSON verifies JSON marshaling
func TestMarshalJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	bytes, err := MarshalJSON(data)
	if err != nil {
		t.Errorf("MarshalJSON failed: %v", err)
	}
	if len(bytes) == 0 {
		t.Error("MarshalJSON returned empty bytes")
	}
}

// TestUnmarshalJSON verifies JSON unmarshaling
func TestUnmarshalJSON(t *testing.T) {
	jsonData := []byte(`{"key":"value"}`)
	var result map[string]string
	err := UnmarshalJSON(jsonData, &result)
	if err != nil {
		t.Errorf("UnmarshalJSON failed: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("UnmarshalJSON returned wrong value: %v", result)
	}
}

// TestMustMarshalJSON verifies panic on invalid input
func TestMustMarshalJSON(t *testing.T) {
	// This should work
	data := map[string]string{"key": "value"}
	bytes := MustMarshalJSON(data)
	if len(bytes) == 0 {
		t.Error("MustMarshalJSON returned empty bytes")
	}

	// This should panic - unserializable data
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustMarshalJSON should panic on invalid input")
		}
	}()

	// Channel cannot be marshaled
	MustMarshalJSON(make(chan int))
}

// TestFormatTime verifies time formatting
func TestFormatTime(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	formatted := FormatTime(now)
	expected := "2024-01-15T10:30:00Z"
	if formatted != expected {
		t.Errorf("Expected %s, got: %s", expected, formatted)
	}
}

// TestParseTime verifies time parsing
func TestParseTime(t *testing.T) {
	input := "2024-01-15T10:30:00Z"
	parsed, err := ParseTime(input)
	if err != nil {
		t.Errorf("ParseTime failed: %v", err)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !parsed.Equal(expected) {
		t.Errorf("Expected %v, got: %v", expected, parsed)
	}
}

// TestNow verifies Now returns UTC time
func TestNow(t *testing.T) {
	now := Now()
	if now.Location() != time.UTC {
		t.Error("Now() should return UTC time")
	}
}

// TestStringPtr verifies StringPtr
func TestStringPtr(t *testing.T) {
	s := "test"
	ptr := StringPtr(s)
	if ptr == nil {
		t.Error("StringPtr returned nil")
	}
	if *ptr != s {
		t.Errorf("Expected %s, got: %s", s, *ptr)
	}
}

// TestStringValue verifies StringValue
func TestStringValue(t *testing.T) {
	// Non-nil pointer
	s := "test"
	if StringValue(&s) != "test" {
		t.Error("StringValue returned wrong value")
	}

	// Nil pointer
	if StringValue(nil) != "" {
		t.Error("StringValue should return empty string for nil")
	}
}

// TestContains verifies Contains
func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !Contains(slice, "b") {
		t.Error("Contains should return true for existing item")
	}

	if Contains(slice, "d") {
		t.Error("Contains should return false for non-existing item")
	}
}

// TestRemove verifies Remove
func TestRemove(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := Remove(slice, "b")

	if len(result) != 2 {
		t.Errorf("Expected length 2, got: %d", len(result))
	}

	if Contains(result, "b") {
		t.Error("Remove should remove the item")
	}
}

// TestFilter verifies Filter
func TestFilter(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	result := Filter(slice, func(n int) bool {
		return n > 2
	})

	if len(result) != 3 {
		t.Errorf("Expected length 3, got: %d", len(result))
	}

	for _, n := range result {
		if n <= 2 {
			t.Error("Filter returned wrong items")
		}
	}
}

// TestMap verifies Map
func TestMap(t *testing.T) {
	slice := []int{1, 2, 3}
	result := Map(slice, func(n int) int {
		return n * 2
	})

	expected := []int{2, 4, 6}
	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got: %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Expected %d at index %d, got: %d", expected[i], i, v)
		}
	}
}
