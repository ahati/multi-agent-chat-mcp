// Package common provides shared utilities following DRY principle
package common

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Common errors used across packages
var (
	ErrEmptyName     = errors.New("name cannot be empty")
	ErrNameTooLong   = errors.New("name exceeds maximum length of 64 characters")
	ErrInvalidName   = errors.New("name contains invalid characters")
	ErrEmptyTaskID   = errors.New("task ID cannot be empty")
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidState  = errors.New("invalid state transition")
	ErrTimeout       = errors.New("operation timed out")
	ErrConnection    = errors.New("connection error")
)

// IDPrefix defines prefixes for different ID types
type IDPrefix string

const (
	PrefixAgent   IDPrefix = "agent"
	PrefixMessage IDPrefix = "msg"
	PrefixTask    IDPrefix = "task"
	PrefixTodo    IDPrefix = "todo"
)

// GenerateID creates a unique ID with the given prefix
// Used by: identity, messaging, taskboard packages (DRY)
func GenerateID(prefix IDPrefix) string {
	b := make([]byte, 8)
	rand.Read(b)
	timestamp := time.Now().UTC().Format("20060102")
	randomPart := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s", prefix, timestamp, randomPart)
}

// GenerateAgentID creates a unique agent ID
func GenerateAgentID() string {
	return GenerateID(PrefixAgent)
}

// GenerateMessageID creates a unique message ID
func GenerateMessageID() string {
	return GenerateID(PrefixMessage)
}

// GenerateTaskID creates a unique task ID
func GenerateTaskID() string {
	return GenerateID(PrefixTask)
}

// GenerateTodoID creates a unique todo ID
func GenerateTodoID() string {
	return GenerateID(PrefixTodo)
}

// Name validation regex
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+$`)

// MaxNameLength defines the maximum allowed name length
const MaxNameLength = 64

// ValidateAgentName validates an agent display name
// Used by: identity, registry packages (DRY)
func ValidateAgentName(name string) error {
	if name == "" {
		return ErrEmptyName
	}
	if len(name) > MaxNameLength {
		return ErrNameTooLong
	}
	if !validNameRegex.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}

// ValidateTaskID validates a task ID
// Used by: taskboard, messaging packages (DRY)
func ValidateTaskID(id string) error {
	if id == "" {
		return ErrEmptyTaskID
	}
	return nil
}

// MarshalJSON serializes a value to JSON with consistent formatting
// Used across all packages for serialization (DRY)
func MarshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON deserializes JSON to a value
// Used across all packages for deserialization (DRY)
func UnmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// MustMarshalJSON serializes a value to JSON, panics on error
// Useful for tests where errors are unexpected
func MustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}

// FormatTime formats a time in a consistent way
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseTime parses a time string formatted by FormatTime
func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// Now returns the current UTC time
func Now() time.Time {
	return time.Now().UTC()
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// StringValue returns the value of a string pointer, or empty if nil
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Contains checks if a string slice contains a value
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Remove removes an item from a string slice
func Remove(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// Filter filters a slice based on a predicate
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms a slice using a mapper function
func Map[T, R any](slice []T, mapper func(T) R) []R {
	result := make([]R, len(slice))
	for i, item := range slice {
		result[i] = mapper(item)
	}
	return result
}
