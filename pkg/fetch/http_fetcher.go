package fetch

import (
	"context"
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
	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName)
	if err != nil {
		return nil, err
	}

	// TODO: Address the following fields
	// timeout := ptypes.Duration(req.timeout)
	// oldestContentAccepted := ptypes.Timestamp(req.oldestContentAccepted)
	expectedDigest, checksumFunction, err := getChecksumSri(req.Qualifiers)
	if err != nil {
		return nil, err
	}

	auth, err := getAuthHeaders(req.Uris, req.Qualifiers)
	if err != nil {
		return nil, err
	}

	for _, uri := range req.Uris {
		buffer, digest := hf.downloadBlob(ctx, uri, digestFunction, auth)
		if _, err = buffer.GetSizeBytes(); err != nil {
			log.Printf("Error downloading blob with URI %s: %v", uri, err)
			continue
		}

		// Check the checksum.sri qualifier, if there's an expected Digest
		if expectedDigest != "" {
			if ok, err := validateChecksumSri(buffer, checksumFunction, expectedDigest); !ok {
				return nil, err
			}
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
	toRemove := qualifier.NewSet([]string{"checksum.sri", QualifierLegacyBazelHTTPHeaders, "bazel.canonical_id"})
	for name := range qualifiers {
		if strings.HasPrefix(name, QualifierHTTPHeaderPrefix) || strings.HasPrefix(name, QualifierHTTPHeaderURLPrefix) {
			toRemove.Add(name)
		}
	}
	return qualifier.Difference(qualifiers, toRemove)
}

// validateChecksumSri ensures that the checksum of the passed response matches the expected value.
func validateChecksumSri(buf buffer.Buffer, checksumFunction bb_digest.Function, expectedDigest string) (bool, error) {
	sizeBytes, err := buf.GetSizeBytes()
	if err != nil {
		return false, err
	}
	checksumGenerator := checksumFunction.NewGenerator(sizeBytes)
	written, err := io.Copy(checksumGenerator, buf.ToReader())
	if err != nil {
		return false, err
	}
	if written != sizeBytes {
		return false, status.Errorf(codes.Internal, "Failed to hash entire buffer")
	}

	checksum := checksumGenerator.Sum().GetProto().GetHash()
	if checksum != expectedDigest {
		return false, status.Errorf(codes.Internal, "Fetched content did not match checksum.sri qualifier: Expected %s, Got %s", expectedDigest, checksum)
	}

	return true, nil
}

// downloadBlob performs the actual blob download, yielding a buffer of the content and its Digest
func (hf *httpFetcher) downloadBlob(ctx context.Context, uri string, digestFunction bb_digest.Function, auth *AuthHeaders) (buffer.Buffer, bb_digest.Digest) {
	// Generate the HTTP Request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to create HTTP request")), bb_digest.BadDigest
	}
	if auth != nil {
		auth.ApplyHeaders(uri, req)
	}

	// Perform the request, check for status
	resp, err := hf.httpClient.Do(req)
	if err != nil {
		log.Printf("Error downloading blob with URI %s: %v", uri, err)
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "HTTP request failed")), bb_digest.BadDigest
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error downloading blob with URI %s: %v", uri, resp.StatusCode)
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest
	}

	// Compute the Digest
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to read response body")), bb_digest.BadDigest
	}
	err = resp.Body.Close()
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to close response body")), bb_digest.BadDigest
	}
	hasher := digestFunction.NewGenerator(resp.ContentLength)
	hasher.Write(bodyBytes)
	digest := hasher.Sum()

	return buffer.NewCASBufferFromByteSlice(digest, bodyBytes, buffer.UserProvided), digest
}

// getChecksumSri parses the checksum.sri qualifier into an expected digest and a digest function to use
func getChecksumSri(qualifiers []*remoteasset.Qualifier) (string, bb_digest.Function, error) {
	hashTypes := map[string]remoteexecution.DigestFunction_Value{
		"sha256":     remoteexecution.DigestFunction_SHA256,
		"sha1":       remoteexecution.DigestFunction_SHA1,
		"md5":        remoteexecution.DigestFunction_MD5,
		"sha384":     remoteexecution.DigestFunction_SHA384,
		"sha512":     remoteexecution.DigestFunction_SHA512,
		"sha256tree": remoteexecution.DigestFunction_SHA256TREE,
	}
	expectedDigest := ""
	digestFunctionEnum := remoteexecution.DigestFunction_UNKNOWN
	for _, qualifier := range qualifiers {
		if qualifier.Name == "checksum.sri" {
			if digestFunctionEnum != remoteexecution.DigestFunction_UNKNOWN {
				return "", bb_digest.Function{}, status.Errorf(codes.InvalidArgument, "Multiple checksum.sri provided")
			}
			parts := strings.SplitN(qualifier.Value, "-", 2)
			if len(parts) != 2 {
				return "", bb_digest.Function{}, status.Errorf(codes.InvalidArgument, "Bad checksum.sri hash expression: %s", qualifier.Value)
			}
			hashName := parts[0]
			b64hash := parts[1]

			digestFunctionEnum, ok := hashTypes[hashName]
			if !ok {
				return "", bb_digest.Function{}, status.Errorf(codes.InvalidArgument, "Unsupported checksum algorithm %s", hashName)
			}

			// Convert expected digest to hex
			decoded, err := base64.StdEncoding.DecodeString(b64hash)
			if err != nil {
				return "", bb_digest.Function{}, status.Errorf(codes.InvalidArgument, "Failed to decode checksum as base64 encoded %s sum: %s", hashName, err.Error())
			}
			expectedDigest = hex.EncodeToString(decoded)

			// Convert to a proper digest function.
			// Note: The Instance name doesn't matter here, this function is used only
			// to give us a convenient API when actually checking the checksum.
			instance := bb_digest.MustNewInstanceName("")
			checksumFunction, err := instance.GetDigestFunction(digestFunctionEnum, len(expectedDigest))
			if err != nil {
				return "", bb_digest.Function{}, status.Errorf(codes.InvalidArgument, "Failed to get checksum function for checksum.sri: %s", err.Error())
			}
			return expectedDigest, checksumFunction, nil
		}
	}

	return "", bb_digest.Function{}, nil
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
