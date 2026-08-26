package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-storage/pkg/auth"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// AuthorizingPushServer decorates a PushServer and validates the requests against an Authorizer
type AuthorizingPushServer struct {
	remoteasset.PushServer
	authorizer auth.Authorizer
}

// NewAuthorizingPushServer wraps a PushServer into an AuthorizingPushServer
func NewAuthorizingPushServer(p remoteasset.PushServer, authorizer auth.Authorizer) *AuthorizingPushServer {
	return &AuthorizingPushServer{
		p,
		authorizer,
	}
}

// PushBlob authorizes a PushBlob request and, if successful, passes it along to the wrapped PushServer
func (ap *AuthorizingPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	if err = auth.AuthorizeSingleInstanceName(ctx, ap.authorizer, instanceName); err != nil {
		return nil, err
	}
	return ap.PushServer.PushBlob(ctx, req)
}

// PushDirectory authorizes a PushDirectory request and, if successful, passes it along to the wrapped PushServer
func (ap *AuthorizingPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	if err = auth.AuthorizeSingleInstanceName(ctx, ap.authorizer, instanceName); err != nil {
		return nil, err
	}
	return ap.PushServer.PushDirectory(ctx, req)
}
