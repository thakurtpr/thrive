package secrets

// SecretStore is the contract for managing encrypted secrets by name.
type SecretStore interface {
	// Create stores a new secret value under the given name and returns its UUID.
	Create(name, value string) (id string, err error)
	// Get decrypts and returns the value of a named secret.
	Get(name string) (string, error)
	// List returns the names of all stored secrets.
	List() ([]string, error)
	// Delete removes a named secret permanently.
	Delete(name string) error
}
