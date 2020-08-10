package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"


	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type httpFetcher struct {
	httpClient                blobstore.HTTPClient
	contentAddressableStorage blobstore.BlobAccess
	allowUpdatesForInstances  map[bb_digest.InstanceName]bool
}

// NewHttpFetcher creates a remoteasset FetchServer compatible service for handling requests which involve downloading
// assets over HTTP and storing them into a CAS.
func NewHttpFetcher(httpClient blobstore.HTTPClient,
	contentAddressableStorage blobstore.BlobAccess,
	allowUpdatesForInstances map[bb_digest.InstanceName]bool) remoteasset.FetchServer {
	return &httpFetcher{
		httpClient:                httpClient,
		contentAddressableStorage: contentAddressableStorage,
		allowUpdatesForInstances:  allowUpdatesForInstances,
	}
}

func (hf *httpFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	var err error
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	if hf.allowUpdatesForInstances[instanceName] == false {
		return nil, status.Errorf(codes.PermissionDenied, "This instance is not permitted to update the CAS.")
	}

	// TODO: Address the following fields
	// timeout := ptypes.Duration(req.timeout)
	// oldestContentAccepted := ptypes.Timestamp(req.oldestContentAccepted)
	expectedDigest, err := getChecksumSri(req.Qualifiers)
	if err != nil {
		return nil, err
	}

	for _, uri := range req.Uris {

		buffer, digest := hf.DownloadBlob(ctx, uri, instanceName, expectedDigest)
		if _, err = buffer.GetSizeBytes(); err != nil {
			continue
		}

		if err := hf.contentAddressableStorage.Put(ctx, digest, buffer); err != nil {
			return nil, util.StatusWrapWithCode(err, codes.Internal, "Failed to place blob into CAS")
		}
		return &remoteasset.FetchBlobResponse{
			Status:     status.New(codes.OK, "Blob fetched successfully!").Proto(),
			Uri:        uri,
			Qualifiers: req.Qualifiers,
			BlobDigest: digest.GetProto(),
		}, nil
	}

	return nil, util.StatusWrapWithCode(err, codes.NotFound, "Unable to download blob from any provided URI")
}

func (hf *httpFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return nil, status.Errorf(codes.PermissionDenied, "HTTP Fetching of directories is not supported!")
}

func (hf *httpFetcher) DownloadBlob(ctx context.Context, uri string, instanceName bb_digest.InstanceName, expectedDigest string) (buffer.Buffer, bb_digest.Digest) {
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
	hexHash := hex.EncodeToString(hash)

	if expectedDigest != "" && hexHash != expectedDigest {
		return buffer.NewBufferFromError(
			status.Errorf(codes.PermissionDenied, "Checksum invalid for fetched blob. Expected: %s, Found: %s", expectedDigest, hexHash)), bb_digest.BadDigest
	}

	digest, err := instanceName.NewDigest(hexHash, int64(nBytes))
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Digest Creation failed")), bb_digest.BadDigest
	}

	return buffer.NewCASBufferFromReader(digest, ioutil.NopCloser(bytes.NewBuffer(body)), buffer.Irreparable), digest
}

func getChecksumSri(qualifiers []*remoteasset.Qualifier) (string, error) {
	for _, qualifier := range qualifiers {
		if qualifier.Name == "checksum.sri" {
			if strings.HasPrefix(qualifier.Value, "sha256-") {
				b64hash := strings.TrimPrefix(qualifier.Value, "sha256-")
				decoded, err := base64.StdEncoding.DecodeString(b64hash)
				if err != nil {
					return "", status.Errorf(codes.InvalidArgument, "Failed to decode checksum as b64 encoded sha256 sum: %s", err.Error())
				}
				return hex.EncodeToString(decoded), nil
			} else {
				return "", status.Errorf(codes.InvalidArgument, "Non sha256 checksums are not supported")
			}
		}
	}
	return "", nil
}
