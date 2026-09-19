package registry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ListRepoTags returns fully-qualified refs for every tag in ref's
// repository (docker pull --all-tags parity).
func ListRepoTags(ctx context.Context, ref, username, password string) ([]string, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("registry: parse reference: %w", err)
	}
	repo := parsed.Context()
	var auth authn.Authenticator
	if username != "" {
		auth = &authn.Basic{Username: username, Password: password}
	}
	tags, err := remote.List(repo, remote.WithAuth(auth), remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("registry: list tags: %w", err)
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, repo.Name()+":"+t)
	}
	return out, nil
}
