package fetch

import (
	"bytes"
	"context"
	"encoding/hex"
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
	var err error
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
		return &remoteasset.FetchBlobResponse{
			Status: 	status.New(codes.OK, "Blob fetched successfully!").Proto(),
			Uri: 		uri,
			Qualifiers: req.Qualifiers,
			BlobDigest: digest.GetProto(),
		}, nil
	}

	return &remoteasset.FetchBlobResponse{
		Status: status.Convert(err).Proto(),
	}, nil
}

func (hf *httpFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
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
	if resp.StatusCode != http.StatusOK {
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest
	}

	// Read all of the content (Not ideal) | // TODO: find a way to avoid internal buffering here
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to read response body")), bb_digest.BadDigest
	}
	nBytes := len(body)

	hasher := sha256.New()
	hasher.Write(body)
	hash := hasher.Sum(nil)

	digest, err := instanceName.NewDigest(hex.EncodeToString(hash), int64(nBytes))
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Digest Creation failed")), bb_digest.BadDigest
	}

	return buffer.NewCASBufferFromReader(digest, ioutil.NopCloser(bytes.NewBuffer(body)), buffer.Irreparable), digest
}