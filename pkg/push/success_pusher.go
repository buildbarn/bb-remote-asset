package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
)

type successPusher struct{}

// NewSuccessPusher returns a Push Server that does nothing and returns successes
func NewSuccessPusher() remoteasset.PushServer {
	return &successPusher{}
}

func (sp *successPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	return &remoteasset.PushBlobResponse{}, nil
}

func (sp *successPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	return &remoteasset.PushDirectoryResponse{}, nil
}
