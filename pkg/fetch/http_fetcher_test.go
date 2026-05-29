package fetch_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"testing/iotest"

	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/testutil"
	"github.com/buildbarn/bb-storage/pkg/util"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type headerMatcher struct {
	headers map[string]string
}

func (hm *headerMatcher) String() string {
	return fmt.Sprintf("has headers: %v", hm.headers)
}

func (hm *headerMatcher) Matches(x interface{}) bool {
	req, ok := x.(*http.Request)
	if !ok {
		return false
	}

	for header, val := range hm.headers {
		headerList, ok := req.Header[header]
		if !ok {
			return false
		}

		if headerList[0] != val {
			return false
		}
	}

	return true
}

// Instance name used in the test
const InstanceName = ""

// Data used as the blob
const TestData = "Hello"

// consumePut mimics a real BlobAccess.Put: it reads the buffer to completion,
// validating the content (as the fast path's in-stream check does) and
// releasing the body/temporary file that would otherwise leak in these mocked tests.
func consumePut(_ context.Context, _ digest.Digest, b buffer.Buffer) error {
	_, err := b.ToByteSlice(1 << 20)
	return err
}

// Convert DigestFunction Enum to strings
var HashNames = map[remoteexecution.DigestFunction_Value]string{
	remoteexecution.DigestFunction_SHA256:     "sha256",
	remoteexecution.DigestFunction_SHA1:       "sha1",
	remoteexecution.DigestFunction_MD5:        "md5",
	remoteexecution.DigestFunction_SHA384:     "sha384",
	remoteexecution.DigestFunction_SHA512:     "sha512",
	remoteexecution.DigestFunction_SHA256TREE: "sha256tree",
}

// Convert a Digest to the representation used by checksum.sri qualifiers.  Note,
// df must match the value used by d
func digestToChecksumSri(df remoteexecution.DigestFunction_Value, d digest.Digest) string {
	return fmt.Sprintf("%s-%s", HashNames[df], base64.StdEncoding.EncodeToString(d.GetHashBytes()))
}

// TestHTTPFetcherFetchBlobFastPathDigestFunctions checks the direct-streaming
// fast path for every checksum.sri digest function: the response is streamed
// straight to the CAS under a digest derived from the checksum.
func TestHTTPFetcherFetchBlobFastPathDigestFunctions(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	for _, digestFunctionEnum := range []remoteexecution.DigestFunction_Value{
		remoteexecution.DigestFunction_SHA256,
		remoteexecution.DigestFunction_SHA1,
		remoteexecution.DigestFunction_MD5,
		remoteexecution.DigestFunction_SHA384,
		remoteexecution.DigestFunction_SHA512,
		remoteexecution.DigestFunction_SHA256TREE,
	} {
		t.Run(digestFunctionEnum.String(), func(t *testing.T) {
			instance := util.Must(digest.NewInstanceName(InstanceName))
			digestFunction, err := instance.GetDigestFunction(digestFunctionEnum, 0)
			require.NoError(t, err)
			digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
			digestGenerator.Write([]byte(TestData))
			helloDigest := digestGenerator.Sum()

			casBlobAccess := mock.NewMockBlobAccess(ctrl)
			roundTripper := mock.NewMockRoundTripper(ctrl)
			HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, "")

			request := &remoteasset.FetchBlobRequest{
				InstanceName:   InstanceName,
				Uris:           []string{"www.example.com"},
				Qualifiers:     []*remoteasset.Qualifier{{Name: "checksum.sri", Value: digestToChecksumSri(digestFunctionEnum, helloDigest)}},
				DigestFunction: digestFunctionEnum,
			}
			body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
			httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
				Status: "200 Success", StatusCode: 200, Body: body, ContentLength: 5,
			}, nil)
			casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(consumePut).After(httpDoCall)

			response, err := HTTPFetcher.FetchBlob(ctx, request)
			require.NoError(t, err)
			require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
			require.Equal(t, response.Status.Code, int32(codes.OK))
		})
	}
}

// TestHTTPFetcherFetchBlobSuccess checks that each download strategy stores the
// blob and returns its digest.
func TestHTTPFetcherFetchBlobSuccess(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := util.Must(digest.NewInstanceName(InstanceName))
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_SHA256, 0)
	require.NoError(t, err)
	digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
	digestGenerator.Write([]byte(TestData))
	helloDigest := digestGenerator.Sum()
	withChecksum := []*remoteasset.Qualifier{{
		Name:  "checksum.sri",
		Value: digestToChecksumSri(remoteexecution.DigestFunction_SHA256, helloDigest),
	}}

	for _, tc := range []struct {
		name          string
		onDisk        bool
		qualifiers    []*remoteasset.Qualifier
		contentLength int64
	}{
		{"DirectStream", false, withChecksum, 5},
		{"BufferInMemory", false, nil, 5},
		{"BufferOnDisk", true, nil, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			casBlobAccess := mock.NewMockBlobAccess(ctrl)
			roundTripper := mock.NewMockRoundTripper(ctrl)
			var downloadDir string
			if tc.onDisk {
				downloadDir = t.TempDir()
			}
			HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, downloadDir)

			request := &remoteasset.FetchBlobRequest{
				InstanceName:   InstanceName,
				Uris:           []string{"www.example.com"},
				Qualifiers:     tc.qualifiers,
				DigestFunction: remoteexecution.DigestFunction_SHA256,
			}
			body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
			httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
				Status: "200 Success", StatusCode: 200, Body: body, ContentLength: tc.contentLength,
			}, nil)
			casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(consumePut).After(httpDoCall)

			response, err := HTTPFetcher.FetchBlob(ctx, request)
			require.NoError(t, err)
			require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
			require.Equal(t, response.Status.Code, int32(codes.OK))

			// A blob buffered on disk leaves no temporary file behind once the
			// Buffer has been consumed.
			if tc.onDisk {
				entries, err := os.ReadDir(downloadDir)
				require.NoError(t, err)
				require.Empty(t, entries)
			}
		})
	}
}

// TestHTTPFetcherFetchBlobOnDiskBufferConsumption checks that the file-backed
// Buffer produced by the on-disk path can be consumed through both ways the CAS
// consumes it — a chunk reader (plain ByteStream.Write) and a writer
// (ZSTD-compressed writes) — and that the temporary file is removed in each.
func TestHTTPFetcherFetchBlobOnDiskBufferConsumption(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := util.Must(digest.NewInstanceName(InstanceName))
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_SHA256, 0)
	require.NoError(t, err)
	digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
	digestGenerator.Write([]byte(TestData))
	helloDigest := digestGenerator.Sum()

	// No checksum.sri and no Content-Length forces an on-disk file-backed Buffer.
	request := &remoteasset.FetchBlobRequest{
		InstanceName:   InstanceName,
		Uris:           []string{"www.example.com"},
		DigestFunction: remoteexecution.DigestFunction_SHA256,
	}

	for _, tc := range []struct {
		name    string
		consume func(buffer.Buffer) ([]byte, error)
	}{
		{"ChunkReader", func(b buffer.Buffer) ([]byte, error) {
			r := b.ToChunkReader(0, 2)
			defer r.Close()
			var data []byte
			for {
				chunk, err := r.Read()
				data = append(data, chunk...)
				if err == io.EOF {
					return data, nil
				}
				if err != nil {
					return nil, err
				}
			}
		}},
		{"Writer", func(b buffer.Buffer) ([]byte, error) {
			var w bytes.Buffer
			err := b.IntoWriter(&w)
			return w.Bytes(), err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			downloadDir := t.TempDir()
			casBlobAccess := mock.NewMockBlobAccess(ctrl)
			roundTripper := mock.NewMockRoundTripper(ctrl)
			HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, downloadDir)

			body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
			httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
				Status: "200 Success", StatusCode: 200, Body: body, ContentLength: -1,
			}, nil)
			casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(
				func(_ context.Context, _ digest.Digest, b buffer.Buffer) error {
					data, err := tc.consume(b)
					require.NoError(t, err)
					require.Equal(t, TestData, string(data))
					return nil
				}).After(httpDoCall)

			response, err := HTTPFetcher.FetchBlob(ctx, request)
			require.NoError(t, err)
			require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))

			entries, err := os.ReadDir(downloadDir)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestHTTPFetcherFetchBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := util.Must(digest.NewInstanceName(InstanceName))
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_SHA256, 0)
	require.NoError(t, err)
	digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
	digestGenerator.Write([]byte(TestData))
	helloDigest := digestGenerator.Sum()

	uri := "www.example.com"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: InstanceName,
		Uris:         []string{uri, "www.another.com"},
		Qualifiers: []*remoteasset.Qualifier{
			{
				Name:  "checksum.sri",
				Value: digestToChecksumSri(remoteexecution.DigestFunction_SHA256, helloDigest),
			},
		},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, "")

	t.Run("UnknownChecksumSriAlgo", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri, "www.another.com"},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "checksum.sri",
					Value: "sha0-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
				},
			},
		}

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Unsupported checksum algorithm sha0"), err)
		require.Nil(t, response)
	})

	t.Run("BadChecksumSriAlgo", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri, "www.another.com"},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "checksum.sri",
					Value: "no_dash",
				},
			},
		}

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Bad checksum.sri hash expression: no_dash"), err)
		require.Nil(t, response)
	})

	t.Run("BadChecksumSriBase64Value", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri, "www.another.com"},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "checksum.sri",
					Value: "sha256-no-base64",
				},
			},
		}

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Failed to decode checksum as base64 encoded sha256 sum: illegal base64 data at input byte 2"), err)
		require.Nil(t, response)
	})

	t.Run("OneFailOneSuccess", func(t *testing.T) {
		httpFailCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil)
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		httpSuccessCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil).After(httpFailCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(consumePut).After(httpSuccessCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil).MaxTimes(2)

		_, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NotNil(t, err)
		require.Equal(t, status.Code(err), codes.NotFound)
	})

	t.Run("WithLegacyAuthHeaders", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "bazel.auth_headers",
					Value: `{ "www.example.com": {"Authorization": "Bearer letmein"}}`,
				},
				{
					Name:  "checksum.sri",
					Value: digestToChecksumSri(remoteexecution.DigestFunction_SHA256, helloDigest),
				},
			},
		}
		require.Empty(t, HTTPFetcher.CheckQualifiers(qualifier.QualifiersToSet(request.Qualifiers)))
		matcher := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer letmein",
			},
		}
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		httpDoCall := roundTripper.EXPECT().RoundTrip(matcher).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(consumePut).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("WithAuthHeaders", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{"www.another.com", uri},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "http_header:Authorization",
					Value: `Bearer anothertoken`,
				},
				{
					Name:  "http_header:Accept",
					Value: "application/vnd.docker.distribution.manifest.list.v2+json",
				},
				{
					Name:  "http_header_url:1:Authorization",
					Value: `Bearer letmein1`,
				},
				{
					Name:  "checksum.sri",
					Value: digestToChecksumSri(remoteexecution.DigestFunction_SHA256, helloDigest),
				},
			},
		}
		require.Empty(t, HTTPFetcher.CheckQualifiers(qualifier.QualifiersToSet(request.Qualifiers)))
		matcherReq1 := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer anothertoken",
				"Accept":        "application/vnd.docker.distribution.manifest.list.v2+json",
			},
		}
		matcherReq2 := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer letmein1",
				"Accept":        "application/vnd.docker.distribution.manifest.list.v2+json",
			},
		}
		roundTripper.EXPECT().RoundTrip(matcherReq1).Return(&http.Response{
			Status:     "404 NotFound",
			StatusCode: 404,
		}, nil)
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		httpDoCall2 := roundTripper.EXPECT().RoundTrip(matcherReq2).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)

		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).DoAndReturn(consumePut).After(httpDoCall2)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

// TestHTTPFetcherFetchBlobBuffering covers the error and cleanup behaviour of
// the buffering download paths (i.e. when the blob can't be streamed directly).
func TestHTTPFetcherFetchBlobBuffering(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := util.Must(digest.NewInstanceName(InstanceName))
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_SHA256, 0)
	require.NoError(t, err)
	digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
	digestGenerator.Write([]byte(TestData))
	helloDigest := digestGenerator.Sum()

	uri := "www.example.com"
	// No checksum.sri and no Content-Length, so the fast path never applies and
	// the body is always buffered.
	request := &remoteasset.FetchBlobRequest{
		InstanceName:   InstanceName,
		Uris:           []string{uri},
		DigestFunction: remoteexecution.DigestFunction_SHA256,
	}

	t.Run("OnDiskReadErrorRemovesTempFile", func(t *testing.T) {
		downloadDir := t.TempDir()
		casBlobAccess := mock.NewMockBlobAccess(ctrl)
		roundTripper := mock.NewMockRoundTripper(ctrl)
		HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, downloadDir)

		// The body fails mid-download; no Put should happen, and the partially
		// written temporary file must be cleaned up.
		body := io.NopCloser(iotest.ErrReader(fmt.Errorf("connection reset")))
		roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status: "200 Success", StatusCode: 200, Body: body, ContentLength: -1,
		}, nil)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, response)
		require.Equal(t, codes.NotFound, status.Code(err))

		entries, err := os.ReadDir(downloadDir)
		require.NoError(t, err)
		require.Empty(t, entries, "partially written temporary file should have been removed")
	})

	t.Run("PutFailureReturnsInternal", func(t *testing.T) {
		casBlobAccess := mock.NewMockBlobAccess(ctrl)
		roundTripper := mock.NewMockRoundTripper(ctrl)
		HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, "")

		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status: "200 Success", StatusCode: 200, Body: body, ContentLength: -1,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(status.Error(codes.Unavailable, "CAS unavailable")).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, response)
		require.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ChecksumMismatch", func(t *testing.T) {
		// checksum.sri present but the body doesn't match it; with no
		// Content-Length the fast path can't apply, so the mismatch is caught
		// after buffering.
		casBlobAccess := mock.NewMockBlobAccess(ctrl)
		roundTripper := mock.NewMockRoundTripper(ctrl)
		HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, "")

		request := &remoteasset.FetchBlobRequest{
			InstanceName:   InstanceName,
			Uris:           []string{uri},
			Qualifiers:     []*remoteasset.Qualifier{{Name: "checksum.sri", Value: digestToChecksumSri(remoteexecution.DigestFunction_SHA256, helloDigest)}},
			DigestFunction: remoteexecution.DigestFunction_SHA256,
		}
		body := io.NopCloser(bytes.NewBuffer([]byte("Not the expected content")))
		roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status: "200 Success", StatusCode: 200, Body: body, ContentLength: -1,
		}, nil)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, response)
		require.Equal(t, codes.Internal, status.Code(err))
	})
}

func TestHTTPFetcherFetchDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "www.example.com"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess, "")
	_, err := HTTPFetcher.FetchDirectory(ctx, request)
	require.NotNil(t, err)
	require.Equal(t, status.Code(err), codes.PermissionDenied)
}
