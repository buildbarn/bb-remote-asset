package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validatingPusher struct {
	pusher remoteasset.PushServer
}

// NewValidatingPusher creates a new Push Server that validates requests before forwarding them to another server
func NewValidatingPusher(pusher remoteasset.PushServer) remoteasset.PushServer {
	return &validatingPusher{
		pusher: pusher,
	}
}

func (vp *validatingPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "PushBlob does not support requests without any URIs specified")
	}
	return vp.pusher.PushBlob(ctx, req)
}

func (vp *validatingPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "PushDirectory does not support requests without any URIs specified")
	}
	return vp.pusher.PushDirectory(ctx, req)
}
