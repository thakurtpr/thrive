//go:build linux
// +build linux

package telemetry

import (
	"errors"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestLogger_ConcurrentSafe verifies that calling Logger() from many goroutines
// simultaneously does not trigger a data race and always returns a non-nil logger.
func TestLogger_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			l := Logger()
			if l == nil {
				t.Error("Logger() returned nil")
			}
		}()
	}

	wg.Wait()
}

// TestInit_ThenLogger_SameInstance verifies that Init() and Logger() converge
// on the same *zap.Logger pointer — subsequent calls must return the same value.
func TestInit_ThenLogger_SameInstance(t *testing.T) {
	// Init is idempotent; calling it in tests is safe.
	if err := Init(); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}

	// Act
	first := Logger()
	second := Logger()

	// Assert
	if first != second {
		t.Error("Logger() returned different pointers on successive calls")
	}
	if first == nil {
		t.Error("Logger() returned nil")
	}
}

// TestFieldHelpers verifies that the convenience field constructors return
// zap.Field values with the expected key names.
func TestFieldHelpers(t *testing.T) {
	cases := []struct {
		name  string
		field zap.Field
		key   string
	}{
		{
			name:  "FieldString",
			field: FieldString("mystr", "value"),
			key:   "mystr",
		},
		{
			name:  "FieldInt",
			field: FieldInt("myint", 42),
			key:   "myint",
		},
		{
			name:  "FieldError",
			field: FieldError(errors.New("boom")),
			key:   "error",
		},
		{
			name:  "FieldBool",
			field: FieldBool("mybool", true),
			key:   "mybool",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.field.Key != tc.key {
				t.Errorf("Key: got %q, want %q", tc.field.Key, tc.key)
			}
		})
	}
}
