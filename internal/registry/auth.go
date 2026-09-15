// Package registry implements image-distribution operations: registry
// authentication, save/load archives, search, and manifest management.
// All code here is portable (no build tags) so the CLI can use it natively
// on Linux and macOS; Windows routes store-backed ops via the VM daemon.
package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

// Credentials holds a registry username/password pair.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// authFile returns the credential-store path (~/.thrive/auth.json, 0600).
// authFileOverride is consulted first so tests can redirect the store.
var authFileOverride string

// manifestStoreOverride redirects the manifest-list store in tests.
var manifestStoreOverride string

func authFile() (string, error) {
	if authFileOverride != "" {
		return authFileOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("registry: home dir: %w", err)
	}
	return filepath.Join(home, ".thrive", "auth.json"), nil
}

func readAuthFile() (map[string]Credentials, error) {
	path, err := authFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Credentials{}, nil
		}
		return nil, fmt.Errorf("registry: read auth file: %w", err)
	}
	var store map[string]Credentials
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("registry: parse auth file: %w", err)
	}
	if store == nil {
		store = map[string]Credentials{}
	}
	return store, nil
}

// RegistryHost normalises a reference to its credential-store key.
// docker.io maps to https://index.docker.io/v1/ (Docker convention);
// all other registries use their hostname. Bare hostnames, URLs, and
// values that are not valid image references pass through (or are
// reduced to) their hostname.
func RegistryHost(ref string) string {
	if strings.Contains(ref, "://") {
		if u, err := url.Parse(ref); err == nil && u.Host != "" {
			if u.Host == "index.docker.io" || u.Host == "docker.io" {
				return "https://index.docker.io/v1/"
			}
			return u.Host
		}
		return ref
	}
	if !strings.Contains(ref, "/") {
		// Single-component names without dots/ports are Docker Hub
		// repositories (e.g. "alpine"); anything else is already a host.
		if !strings.ContainsAny(ref, ".:") {
			return "https://index.docker.io/v1/"
		}
		return ref
	}
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return ref
	}
	host := parsed.Context().RegistryStr()
	if host == name.DefaultRegistry {
		return "https://index.docker.io/v1/"
	}
	return host
}

// Login stores credentials for the registry hosting ref (or raw host).
func Login(server, username, password string) error {
	if username == "" {
		return fmt.Errorf("registry: username required")
	}
	host := RegistryHost(server)
	store, err := readAuthFile()
	if err != nil {
		return err
	}
	store[host] = Credentials{Username: username, Password: password}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshal auth: %w", err)
	}
	path, err := authFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("registry: mkdir: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("registry: write auth: %w", err)
	}
	return nil
}

// Logout removes stored credentials for a registry host (or image ref).
func Logout(server string) error {
	host := RegistryHost(server)
	store, err := readAuthFile()
	if err != nil {
		return err
	}
	if _, ok := store[host]; !ok {
		// Fall back to the literal key for stores written by older versions.
		if _, ok := store[server]; ok {
			host = server
		} else {
			return fmt.Errorf("registry: not logged in to %s", server)
		}
	}
	delete(store, host)
	data, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("registry: marshal auth: %w", err)
	}
	path, err := authFile()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("registry: write auth: %w", err)
	}
	return nil
}

// StoredAuth returns saved credentials for a registry host, or nil.
func StoredAuth(server string) *Credentials {
	store, err := readAuthFile()
	if err != nil {
		return nil
	}
	if c, ok := store[server]; ok {
		return &c
	}
	return nil
}

// ResolveAuth returns an authenticator for ref: explicit flags win,
// then the credential store, then anonymous.
func ResolveAuth(ref, flagUser, flagPass string) authn.Authenticator {
	if flagUser != "" {
		return &authn.Basic{Username: flagUser, Password: flagPass}
	}
	if c := StoredAuth(RegistryHost(ref)); c != nil {
		return &authn.Basic{Username: c.Username, Password: c.Password}
	}
	return authn.Anonymous
}

// BasicAuthHeader builds a Docker-style base64 auth header value.
func BasicAuthHeader(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}
