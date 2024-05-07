package storage

import (
	"context"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/auth"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// AuthorizingAssetStore wraps an asset store and validates requests against the authorizers
type AuthorizingAssetStore struct {
	AssetStore
	fetchAuthorizer auth.Authorizer
	pushAuthorizer  auth.Authorizer
}

// NewAuthorizingAssetStore creates a new authorizing asset store
func NewAuthorizingAssetStore(as AssetStore, fetchAuthorizer, pushAuthorizer auth.Authorizer) *AuthorizingAssetStore {
	return &AuthorizingAssetStore{
		as,
		fetchAuthorizer,
		pushAuthorizer,
	}
}

// Get is a wrapper that validates credentials against FetchAuthorizer
func (aas *AuthorizingAssetStore) Get(ctx context.Context, ref *asset.AssetReference, instanceName digest.InstanceName) (*asset.Asset, error) {
	if err := auth.AuthorizeSingleInstanceName(ctx, aas.fetchAuthorizer, instanceName); err != nil {
		return nil, err
	}
	return aas.AssetStore.Get(ctx, ref, instanceName)
}

// Put is a wrapper that validates credentials against PushAuthorizer
func (aas *AuthorizingAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, instanceName digest.InstanceName) error {
	if err := auth.AuthorizeSingleInstanceName(ctx, aas.pushAuthorizer, instanceName); err != nil {
		return err
	}
	return aas.AssetStore.Put(ctx, ref, data, instanceName)
}
