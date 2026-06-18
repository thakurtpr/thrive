package secrets_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeSecretStore is an in-memory implementation of SecretStore for portable testing.
type fakeSecretStore struct {
	mu      sync.Mutex
	entries map[string]string // name → value
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{entries: make(map[string]string)}
}

func (s *fakeSecretStore) Create(name, value string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[name]; exists {
		return "", fmt.Errorf("secret %q already exists", name)
	}
	s.entries[name] = value
	return uuid.New().String(), nil
}

func (s *fakeSecretStore) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.entries[name]
	if !ok {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return v, nil
}

func (s *fakeSecretStore) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.entries))
	for n := range s.entries {
		names = append(names, n)
	}
	return names, nil
}

func (s *fakeSecretStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[name]; !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	delete(s.entries, name)
	return nil
}

func TestFakeSecretStoreSetGetRoundTrip(t *testing.T) {
	store := newFakeSecretStore()
	id, err := store.Create("DB_PASS", "s3cr3t!")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty UUID")
	}
	val, err := store.Get("DB_PASS")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "s3cr3t!" {
		t.Errorf("expected s3cr3t!, got %q", val)
	}
}

func TestFakeSecretStoreGetNonExistent(t *testing.T) {
	store := newFakeSecretStore()
	if _, err := store.Get("MISSING_KEY_XYZ"); err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
}

func TestFakeSecretStoreListEmpty(t *testing.T) {
	store := newFakeSecretStore()
	names, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestFakeSecretStoreListAfterCreate(t *testing.T) {
	store := newFakeSecretStore()
	_, _ = store.Create("KEY_A", "val1")
	_, _ = store.Create("KEY_B", "val2")
	names, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 secrets, got %d: %v", len(names), names)
	}
}

func TestFakeSecretStoreDelete(t *testing.T) {
	store := newFakeSecretStore()
	_, _ = store.Create("MY_TOKEN", "abc123")
	if err := store.Delete("MY_TOKEN"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("MY_TOKEN"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFakeSecretStoreDuplicateCreateFails(t *testing.T) {
	store := newFakeSecretStore()
	_, err := store.Create("DUPE_KEY", "first-value")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = store.Create("DUPE_KEY", "second-value")
	if err == nil {
		t.Fatal("expected error when creating duplicate secret")
	}
}

func TestFakeSecretStoreDeleteNonExistent(t *testing.T) {
	store := newFakeSecretStore()
	if err := store.Delete("GHOST_KEY"); err == nil {
		t.Fatal("expected error when deleting nonexistent secret")
	}
}

func TestFakeSecretStoreConcurrentAccess(t *testing.T) {
	store := newFakeSecretStore()
	// Pre-populate to avoid race on Create with duplicate detection.
	for i := 0; i < 5; i++ {
		_, _ = store.Create(fmt.Sprintf("KEY_%d", i), fmt.Sprintf("val_%d", i))
	}

	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("KEY_%d", i)
		go func(k string) {
			defer func() { done <- struct{}{} }()
			_, _ = store.Get(k)
			_, _ = store.List()
		}(key)
	}
	for i := 0; i < 5; i++ {
		<-done
	}
}
