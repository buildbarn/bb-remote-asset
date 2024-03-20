package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-storage/pkg/auth"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
)

// AuthorizingFetcher decorates Fetcher and validates the requests against an Authorizer
type AuthorizingFetcher struct {
	Fetcher
	authorizer auth.Authorizer
}

// NewAuthorizingFetcher creates a new AuthorizingFetcher
func NewAuthorizingFetcher(f Fetcher, authorizer auth.Authorizer) *AuthorizingFetcher {
	return &AuthorizingFetcher{
		f,
		authorizer,
	}
}

// FetchBlob wraps FetchBlob requests, validate request against authorizer
func (af *AuthorizingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	if err = auth.AuthorizeSingleInstanceName(ctx, af.authorizer, instanceName); err != nil {
		return nil, err
	}
	return af.Fetcher.FetchBlob(ctx, req)
}

// FetchDirectory wraps FetchDirectory requests, validate request against authorizer
func (af *AuthorizingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	if err = auth.AuthorizeSingleInstanceName(ctx, af.authorizer, instanceName); err != nil {
		return nil, err
	}
	return af.Fetcher.FetchDirectory(ctx, req)
}
