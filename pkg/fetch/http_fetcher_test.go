package fetch_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/testutil"

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

func TestHTTPFetcherFetchBlobSuccessSHA256(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA256,
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA1(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA1,
	)
}

func TestHTTPFetcherFetchBlobSuccessMD5(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_MD5,
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA384(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA384,
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA512(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA512,
	)
}

func TestHTTPFetcherFetchBlobSuccessSha256tree(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA256TREE,
	)
}

func testHTTPFetcherFetchBlobSuccessWithHasher(t *testing.T, digestFunctionEnum remoteexecution.DigestFunction_Value) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := digest.MustNewInstanceName(InstanceName)
	digestFunction, err := instance.GetDigestFunction(digestFunctionEnum, 0)
	require.NoError(t, err)
	digestGenerator := digestFunction.NewGenerator(int64(len(TestData)))
	digestGenerator.Write([]byte(TestData))
	helloDigest := digestGenerator.Sum()

	request := &remoteasset.FetchBlobRequest{
		InstanceName: InstanceName,
		Uris:         []string{"www.example.com"},
		Qualifiers: []*remoteasset.Qualifier{
			{
				Name:  "checksum.sri",
				Value: digestToChecksumSri(digestFunctionEnum, helloDigest),
			},
		},
		DigestFunction: digestFunctionEnum,
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)

	t.Run("Success"+helloDigest.GetDigestFunction().GetEnumValue().String(), func(t *testing.T) {
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoContentLength", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

func TestHTTPFetcherFetchBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instance := digest.MustNewInstanceName(InstanceName)
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
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)

	t.Run("SuccessNoExpectedDigest", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoExpectedDigestOrContentLength", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBuffer([]byte(TestData)))
		request := &remoteasset.FetchBlobRequest{
			InstanceName: InstanceName,
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

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
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpSuccessCall)

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
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

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

		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall2)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
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
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)
	_, err := HTTPFetcher.FetchDirectory(ctx, request)
	require.NotNil(t, err)
	require.Equal(t, status.Code(err), codes.PermissionDenied)
}
