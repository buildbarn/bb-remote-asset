package push

import (
	"context"
	"log"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"google.golang.org/grpc/status"
)

type loggingPusher struct {
	pusher remoteasset.PushServer
}

// NewLoggingPusher creates a new Push server that logs requests before forwarding to another server
func NewLoggingPusher(pusher remoteasset.PushServer) remoteasset.PushServer {
	return &loggingPusher{
		pusher: pusher,
	}
}

func (lp *loggingPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	log.Printf("Pushing Blob %s with qualifiers %s to be %s", req.Uris, req.Qualifiers, req.BlobDigest)
	resp, err := lp.pusher.PushBlob(ctx, req)
	if err == nil {
		log.Printf("PushBlob completed for %s successfully", req.Uris)
	} else {
		log.Printf("PushBlob completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}

func (lp *loggingPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	log.Printf("Pushing Blob %s with qualifiers %s to be %s", req.Uris, req.Qualifiers, req.RootDirectoryDigest)
	resp, err := lp.pusher.PushDirectory(ctx, req)
	if err == nil {
		log.Printf("PushDirectory completed for %s successfully", req.Uris)
	} else {
		log.Printf("PushDirectory completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}
