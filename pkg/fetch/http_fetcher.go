package fetch

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
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
	downloadDirectory         string
}

// NewHTTPFetcher creates a remoteasset FetchServer compatible service for handling requests which involve downloading
// assets over HTTP and storing them into a CAS.
//
// downloadDirectory selects how blobs that can't be streamed directly to the
// CAS are handled: an empty string buffers them in memory (the default); a
// non-empty string buffers them in a temporary file in that directory.
func NewHTTPFetcher(httpClient *http.Client,
	contentAddressableStorage blobstore.BlobAccess,
	downloadDirectory string,
) Fetcher {
	return &httpFetcher{
		httpClient:                httpClient,
		contentAddressableStorage: contentAddressableStorage,
		downloadDirectory:         downloadDirectory,
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
		buffer, digest, checksum := hf.downloadBlob(ctx, uri, digestFunction, checksumFunction, expectedDigest, auth)
		if _, err = buffer.GetSizeBytes(); err != nil {
			log.Printf("Error downloading blob with URI %s: %v", uri, err)
			continue
		}

		// Validate the checksum.sri qualifier. For buffered downloads checksum is
		// the hash computed while reading, so a mismatch is caught here. For the
		// fast path where we store in the CAS, checksum equals the expected hash
		// (the digest is derived from it), making this a no-op; a mismatch there
		// instead surfaces from the Put below as bb-storage's
		// "Buffer has checksum ...".
		if expectedDigest != "" && checksum != expectedDigest {
			buffer.Discard()
			return nil, status.Errorf(codes.Internal, "Fetched content did not match checksum.sri qualifier: Expected %s, Got %s", expectedDigest, checksum)
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

// downloadBlob downloads the blob at uri, returning a Buffer of the content,
// its Digest, and the content's checksum.sri when expectedChecksum is set.
func (hf *httpFetcher) downloadBlob(ctx context.Context, uri string, digestFunction, checksumFunction bb_digest.Function, expectedChecksum string, auth *AuthHeaders) (buffer.Buffer, bb_digest.Digest, string) {
	// Generate the HTTP Request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to create HTTP request")), bb_digest.BadDigest, ""
	}
	if auth != nil {
		auth.ApplyHeaders(uri, req)
	}

	// Perform the request, check for status
	resp, err := hf.httpClient.Do(req)
	if err != nil {
		log.Printf("Error downloading blob with URI %s: %v", uri, err)
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "HTTP request failed")), bb_digest.BadDigest, ""
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error downloading blob with URI %s: %v", uri, resp.StatusCode)
		return buffer.NewBufferFromError(status.Errorf(codes.Internal, "HTTP request failed with status %#v", resp.Status)), bb_digest.BadDigest, ""
	}

	// If the digest and size are known, and the checksum.sri method uses the
	// request's digest function (so its hash IS the CAS hash) we can stream
	// straight to the CAS, validating the content as the Buffer is read.
	if expectedChecksum != "" && checksumFunction.GetEnumValue() == digestFunction.GetEnumValue() && resp.ContentLength >= 0 {
		if digest, err := digestFunction.NewDigest(expectedChecksum, resp.ContentLength); err == nil {
			return buffer.NewCASBufferFromReader(digest, resp.Body, buffer.UserProvided), digest, expectedChecksum
		}
	}

	// Otherwise the body must be read in full to compute the digest. Buffer
	// the content either in memory or on disk, hashing as we go.
	digestGenerator := digestFunction.NewGenerator(resp.ContentLength)
	var hashes io.Writer = digestGenerator
	var checksumGenerator *bb_digest.Generator
	if expectedChecksum != "" {
		checksumGenerator = checksumFunction.NewGenerator(resp.ContentLength)
		hashes = io.MultiWriter(digestGenerator, checksumGenerator)
	}
	body := io.TeeReader(resp.Body, hashes)

	var buf buffer.Buffer
	if hf.downloadDirectory != "" {
		buf, err = bufferBlobOnDisk(hf.downloadDirectory, body)
	} else {
		buf, err = bufferBlobInMemory(body)
	}
	if err != nil {
		resp.Body.Close()
		return buffer.NewBufferFromError(err), bb_digest.BadDigest, ""
	}
	if err := resp.Body.Close(); err != nil {
		buf.Discard()
		return buffer.NewBufferFromError(util.StatusWrapWithCode(err, codes.Internal, "Failed to close response body")), bb_digest.BadDigest, ""
	}

	checksum := ""
	if checksumGenerator != nil {
		checksum = checksumGenerator.Sum().GetProto().GetHash()
	}
	return buf, digestGenerator.Sum(), checksum
}

// bufferBlobInMemory reads r fully into memory and returns a Buffer over the bytes.
func bufferBlobInMemory(r io.Reader) (buffer.Buffer, error) {
	bodyBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, util.StatusWrapWithCode(err, codes.Internal, "Failed to read response body")
	}
	// downloadBlob computes the digest from these same bytes, so bodyBytes
	// already matches it; a validated buffer avoids re-hashing it on upload.
	return buffer.NewValidatedBufferFromByteSlice(bodyBytes), nil
}

// removeOnCloseFile is a temporary file whose Close() also removes it from
// disk, used to back a Buffer with a download buffered on disk.
type removeOnCloseFile struct {
	*os.File
}

func (f *removeOnCloseFile) Close() error {
	err := f.File.Close()
	removeErr := os.Remove(f.File.Name())
	if err != nil {
		return err
	}
	return removeErr
}

// bufferBlobOnDisk writes r to a temporary file in dir and returns a Buffer
// backed by that file.
func bufferBlobOnDisk(dir string, r io.Reader) (buffer.Buffer, error) {
	f, err := os.CreateTemp(dir, "bb-remote-asset-download-*")
	if err != nil {
		return nil, util.StatusWrapWithCode(err, codes.Internal, "Failed to create temporary file for download")
	}
	tmpFile := &removeOnCloseFile{File: f}
	sizeBytes, err := io.Copy(tmpFile, r)
	if err != nil {
		// On success the returned Buffer owns tmpFile; here it doesn't, so
		// close it to remove the partially-written file.
		tmpFile.Close()
		return nil, util.StatusWrapWithCode(err, codes.Internal, "Failed to read response body")
	}
	// downloadBlob computes the digest from these same bytes, so the file
	// already matches it; a validated buffer avoids re-hashing it on upload.
	return buffer.NewValidatedBufferFromReaderAt(tmpFile, sizeBytes), nil
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
			instance := util.Must(bb_digest.NewInstanceName(""))
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
