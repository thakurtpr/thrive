package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/signing"
)

// SignCmd returns the `thrive sign` parent command with subcommands:
// sign image IMAGE, sign keygen, sign verify IMAGE.
func SignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Manage image signatures (sign, verify, keygen)",
	}
	cmd.AddCommand(signImageCmd())
	cmd.AddCommand(signKeygenCmd())
	cmd.AddCommand(VerifyCmd())
	return cmd
}

func signImageCmd() *cobra.Command {
	var keyName string
	cmd := &cobra.Command{
		Use:   "image IMAGE",
		Short: "Sign an image with an Ed25519 key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSign(args[0], keyName)
		},
	}
	cmd.Flags().StringVar(&keyName, "key", "default", "Key pair name (in ~/.thrive/keys/)")
	return cmd
}

func signKeygenCmd() *cobra.Command {
	var keyName string
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate an Ed25519 key pair for image signing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := signing.GenerateKeyPair(keyName); err != nil {
				return fmt.Errorf("keygen: %w", err)
			}
			fmt.Printf("Generated ~/.thrive/keys/%s.{priv,pub}\n", keyName)
			return nil
		},
	}
	cmd.Flags().StringVar(&keyName, "key", "default", "Key pair name")
	return cmd
}

// VerifyCmd is also usable as a standalone `thrive verify` alias.
func VerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify IMAGE",
		Short: "Verify an image's Ed25519 signature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			home, _ := os.UserHomeDir()
			imgDir := filepath.Join(home, ".thrive", "images", image.SafeRef(ref))

			data, err := os.ReadFile(filepath.Join(imgDir, "manifest.json"))
			if err != nil {
				return fmt.Errorf("verify: image %q not found: %w", ref, err)
			}
			var meta struct{ Digest string }
			json.Unmarshal(data, &meta)

			sigPath := signing.SigPath(imgDir, meta.Digest)
			if err := signing.Verify(meta.Digest, sigPath); err != nil {
				return fmt.Errorf("VERIFICATION FAILED: %w", err)
			}
			short := meta.Digest
			if len(short) > 20 {
				short = short[:20] + "..."
			}
			fmt.Printf("OK: signature verified for %s (%s)\n", ref, short)
			return nil
		},
	}
}

func runSign(ref, keyName string) error {
	home, _ := os.UserHomeDir()
	imgDir := filepath.Join(home, ".thrive", "images", image.SafeRef(ref))

	data, err := os.ReadFile(filepath.Join(imgDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("sign: image %q not found locally (run thrive pull first): %w", ref, err)
	}
	var meta struct{ Digest string }
	json.Unmarshal(data, &meta)
	if meta.Digest == "" {
		return fmt.Errorf("sign: image manifest has no digest")
	}

	kp, err := signing.LoadKeyPair(keyName)
	if err != nil {
		fmt.Printf("sign: key %q not found, generating...\n", keyName)
		if kp, err = signing.GenerateKeyPair(keyName); err != nil {
			return fmt.Errorf("sign: generate key: %w", err)
		}
	}

	sigPath := signing.SigPath(imgDir, meta.Digest)
	sig, err := signing.Sign(meta.Digest, sigPath, kp)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	short := sig.ImageDigest
	if len(short) > 20 {
		short = short[:20] + "..."
	}
	fmt.Printf("Signed %s\n  digest:   %s\n  sig file: %s\n", ref, short, sigPath)
	return nil
}
