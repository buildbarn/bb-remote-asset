package fetch

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"crypto/sha256"
	"net/http"

	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO: Move from bb-storage pkg/blobstore/reference_expanding_blob_access.go into a shared util lib
// HTTPClient is an interface around Go's standard HTTP client type. It
// has been added to aid unit testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpFetcher struct {
	httpClient 				  HTTPClient
	contentAddressableStorage blobstore.BlobAccess
}

func NewHttpFetcher(httpClient HTTPClient, contentAddressableStorage blobstore.BlobAccess) remoteasset.FetchServer {
	return &httpFetcher{
		httpClient:				   httpClient,
		contentAddressableStorage: contentAddressableStorage,
	}
}

func (hf *httpFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}
	// TODO: Address the following fields
	// timeout := ptypes.Duration(req.timeout)
	// oldestContentAccepted := ptypes.Timestamp(req.oldestContentAccepted)

	for _, uri := range req.Uris {
		buffer, digest := hf.DownloadBlob(ctx, uri, instanceName)
		if _, err := buffer.GetSizeBytes(); err != nil {
			continue
		}

		if err := hf.contentAddressableStorage.Put(ctx, digest, buffer); err != nil {
			return &remoteasset.FetchBlobResponse{
				Status: status.Convert(err).Proto(),
			}, nil
		}
	}

	fmt.Println("FetchBlob Request")
	return &remoteasset.FetchBlobResponse{
		Status: status.New(codes.PermissionDenied, "Not supported!").Proto(),
	}, nil
}

func (hf *httpFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	fmt.Println("FetchDirectory Request")
	return &remoteasset.FetchDirectoryResponse{
		Status: status.New(codes.PermissionDenied, "Not supported!").Proto(),
	}, nil
}

func (hf *httpFetcher) DownloadBlob(ctx context.Context, uri string, instanceName bb_digest.InstanceName) (buffer.Buffer, bb_digest.Digest) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to create HTTP request")), bb_digest.BadDigest
	}
	resp, err := hf.httpClient.Do(req)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "HTTP request failed")), bb_digest.BadDigest
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest
	}

	hash := sha256.New()
	// Copy the contents of the body for hashing and consumption by the buffer library
	blobReader :=  io.TeeReader(resp.Body, hash)

	digest, err := instanceName.NewDigest(string(hash.Sum(nil)), resp.ContentLength)
	if err != nil {
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest
	}

	return buffer.NewCASBufferFromReader(digest, ioutil.NopCloser(blobReader), buffer.Irreparable), digest
}