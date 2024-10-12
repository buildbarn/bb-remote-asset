package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"

	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// QualifierLegacyBazelHTTPHeaders is the qualifier older versions of bazel sends.
	QualifierLegacyBazelHTTPHeaders = "bazel.auth_headers"
	// QualifierHTTPHeaderPrefix is a qualifer to add a header to all URIs.
	// Qualifier will be in the form http_header:<header>
	QualifierHTTPHeaderPrefix = "http_header:"
	// QualifierHTTPHeaderURLPrefix is a qualifier to add a header to a specific URI.
	// Qualifier will be in the form http_header_url:<index>:<header>
	QualifierHTTPHeaderURLPrefix = "http_header_url:"
)

type httpFetcher struct {
	httpClient                *http.Client
	contentAddressableStorage blobstore.BlobAccess
}

// NewHTTPFetcher creates a remoteasset FetchServer compatible service for handling requests which involve downloading
// assets over HTTP and storing them into a CAS.
func NewHTTPFetcher(httpClient *http.Client,
	contentAddressableStorage blobstore.BlobAccess,
) Fetcher {
	return &httpFetcher{
		httpClient:                httpClient,
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
	expectedDigest, err := getChecksumSri(req.Qualifiers)
	if err != nil {
		return nil, err
	}

	auth, err := getAuthHeaders(req.Uris, req.Qualifiers)
	if err != nil {
		return nil, err
	}

	for _, uri := range req.Uris {

		buffer, digest := hf.downloadBlob(ctx, uri, instanceName, expectedDigest, auth)
		if _, err = buffer.GetSizeBytes(); err != nil {
			log.Printf("Error downloading blob with URI %s: %v", uri, err)
			continue
		}

		if err = hf.contentAddressableStorage.Put(ctx, digest, buffer); err != nil {
			log.Printf("Error downloading blob with URI %s: %v", uri, err)
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

func (hf *httpFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Difference(qualifiers, qualifier.NewSet([]string{"checksum.sri", "bazel.auth_headers", "bazel.canonical_id"}))
}

func (hf *httpFetcher) downloadBlob(ctx context.Context, uri string, instanceName bb_digest.InstanceName, expectedDigest string, auth *AuthHeaders) (buffer.Buffer, bb_digest.Digest) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to create HTTP request")), bb_digest.BadDigest
	}

	if auth != nil {
		auth.ApplyHeaders(uri, req)
	}

	resp, err := hf.httpClient.Do(req)
	if err != nil {
		log.Printf("Error downloading blob with URI %s: %v", uri, err)
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "HTTP request failed")), bb_digest.BadDigest
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error downloading blob with URI %s: %v", uri, resp.StatusCode)
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest
	}

	// Work out the digest of the downloaded data
	//
	// If the HTTP response includes the content length (indicated by the value
	// of the field being >= 0) and the client has provided an expected hash of
	// the content, we can avoid holding the contents of the entire file in
	// memory at one time by creating a new buffer from the response body
	// directly
	//
	// If either one (or both) of these things is not available, we will need to
	// read the enitre response body into a byte slice in order to be able to
	// determine the digest
	length := resp.ContentLength
	body := resp.Body
	if length < 0 || expectedDigest == "" {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to read response body")), bb_digest.BadDigest
		}
		err = resp.Body.Close()
		if err != nil {
			return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to close response body")), bb_digest.BadDigest
		}
		length = int64(len(bodyBytes))

		// If we don't know what the hash should be we will need to work out the
		// actual hash of the content
		if expectedDigest == "" {
			hasher := sha256.New()
			hasher.Write(bodyBytes)
			hash := hasher.Sum(nil)
			expectedDigest = hex.EncodeToString(hash)
		}

		body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	digestFunction, err := instanceName.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(expectedDigest))
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapfWithCode(err, codes.Internal, "Failed to get digest function for instance: %v", instanceName)), bb_digest.BadDigest
	}
	digest, err := digestFunction.NewDigest(expectedDigest, length)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Digest Creation failed")), bb_digest.BadDigest
	}

	// An error will be generated down the line if the data does not match the
	// digest
	return buffer.NewCASBufferFromReader(digest, body, buffer.UserProvided), digest
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
			}
			return "", status.Errorf(codes.InvalidArgument, "Non sha256 checksums are not supported")
		}
	}
	return "", nil
}

func getAuthHeaders(uris []string, qualifiers []*remoteasset.Qualifier) (*AuthHeaders, error) {
	ah := AuthHeaders{}
	perURLQualifiers := map[string]string{}
	for _, qualifier := range qualifiers {
		// If this is set, then any other headers are ignored
		// as this is the only way to set headers in older versions of bazel
		if qualifier.Name == QualifierLegacyBazelHTTPHeaders {
			return NewAuthHeadersFromQualifier(qualifier.Value)
		}

		if strings.HasPrefix(qualifier.Name, QualifierHTTPHeaderPrefix) {
			header := strings.TrimPrefix(qualifier.Name, QualifierHTTPHeaderPrefix)
			for _, uri := range uris {
				ah.AddHeader(uri, header, qualifier.Value)
			}
		}

		if strings.HasPrefix(qualifier.Name, QualifierHTTPHeaderURLPrefix) {
			perURLQualifiers[qualifier.Name] = qualifier.Value
		}
	}
	// If we have per URL headers, we need to go through and apply them after applying the global headers.
	for k, v := range perURLQualifiers {
		parts := strings.Split(k, ":")
		if len(parts) != 3 {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid http_header_url qualifier: %s", k)
		}
		uriIdx, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid http_header_url qualifier: %s: Bad URL index: %v: %v", k, parts[1], err)
		}
		if uriIdx < 0 || uriIdx >= int64(len(uris)) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid http_header_url qualifier: %s: URL index out of range: %v", k, uriIdx)
		}
		header := parts[2]
		ah.AddHeader(uris[uriIdx], header, v)

	}

	return &ah, nil
}
