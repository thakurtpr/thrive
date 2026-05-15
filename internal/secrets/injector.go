//go:build linux
// +build linux

package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

const containerSecretsPath = "/run/thrive/containers/%s/secrets"

func Inject(containerID string, secretNames []string) error {
	log := telemetry.Logger()
	log.Info("injector.Inject: starting", telemetry.FieldString("containerID", containerID), telemetry.FieldInt("secretCount", len(secretNames)))

	if len(secretNames) == 0 {
		return nil
	}

	if err := loadManifest(); err != nil {
		return fmt.Errorf("injector.Inject: loadManifest: %w", err)
	}

	secretsMountPath := fmt.Sprintf(containerSecretsPath, containerID)
	if err := os.MkdirAll(secretsMountPath, 0700); err != nil {
		return fmt.Errorf("injector.Inject: mkdir: %w", err)
	}

	if err := syscall.Mount("tmpfs", secretsMountPath, "tmpfs", syscall.MS_NOEXEC|syscall.MS_NOSUID, "size=1M"); err != nil {
		log.Error("injector.Inject: mount tmpfs failed", telemetry.FieldString("path", secretsMountPath), telemetry.FieldError(err))
		return fmt.Errorf("injector.Inject: mount: %w", err)
	}

	for _, name := range secretNames {
		manifest.mu.RLock()
		entry, exists := manifest.entries[name]
		manifest.mu.RUnlock()

		if !exists {
			log.Warn("injector.Inject: secret not found, skipping", telemetry.FieldString("name", name))
			continue
		}

		secretPath := filepath.Join(secretsDir, entry.UUID)
		encrypted, err := os.ReadFile(secretPath)
		if err != nil {
			log.Error("injector.Inject: read secret failed", telemetry.FieldString("name", name), telemetry.FieldError(err))
			continue
		}

		plaintext, err := Decrypt(encrypted)
		if err != nil {
			log.Error("injector.Inject: decrypt failed", telemetry.FieldString("name", name), telemetry.FieldError(err))
			continue
		}

		secretFilePath := filepath.Join(secretsMountPath, name)
		if err := os.WriteFile(secretFilePath, plaintext, 0600); err != nil {
			log.Error("injector.Inject: write secret file failed", telemetry.FieldString("name", name), telemetry.FieldError(err))
			continue
		}
		log.Info("injector.Inject: injected secret", telemetry.FieldString("name", name))
	}

	log.Info("injector.Inject: completed", telemetry.FieldString("containerID", containerID))
	return nil
}

func Cleanup(containerID string) error {
	log := telemetry.Logger()
	log.Info("injector.Cleanup: starting", telemetry.FieldString("containerID", containerID))

	secretsMountPath := fmt.Sprintf(containerSecretsPath, containerID)

	if _, err := os.Stat(secretsMountPath); os.IsNotExist(err) {
		return nil
	}

	if err := syscall.Unmount(secretsMountPath, 0); err != nil {
		log.Error("injector.Cleanup: unmount failed", telemetry.FieldString("path", secretsMountPath), telemetry.FieldError(err))
		return fmt.Errorf("injector.Cleanup: unmount: %w", err)
	}

	os.RemoveAll(secretsMountPath)
	log.Info("injector.Cleanup: completed", telemetry.FieldString("containerID", containerID))
	return nil
}