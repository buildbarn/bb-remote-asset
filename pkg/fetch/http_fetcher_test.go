package fetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
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

func TestHTTPFetcherFetchBlobSuccessSHA256(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA256,
		"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969",
		"sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA1(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA1,
		"f7ff9e8b7bb2e09b70935a5d785e0cc5d9d0abf0",
		"sha1-9/+ei3uy4Jtwk1pdeF4MxdnQq/A=",
	)
}

func TestHTTPFetcherFetchBlobSuccessMD5(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_MD5,
		"8b1a9953c4611296a827abf8c47804d7",
		"md5-ixqZU8RhEpaoJ6v4xHgE1w==",
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA384(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA384,
		"3519fe5ad2c596efe3e276a6f351b8fc0b03db861782490d45f7598ebd0ab5fd5520ed102f38c4a5ec834e98668035fc",
		"sha384-NRn+WtLFlu/j4nam81G4/AsD24YXgkkNRfdZjr0Ktf1VIO0QLzjEpeyDTphmgDX8",
	)
}

func TestHTTPFetcherFetchBlobSuccessSHA512(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA512,
		"3615f80c9d293ed7402687f94b22d58e529b8cc7916f8fac7fddf7fbd5af4cf777d3d795a7a00a16bf7e7f3fb9561ee9baae480da9fe7a18769e71886b03f315",
		"sha512-NhX4DJ0pPtdAJof5SyLVjlKbjMeRb4+sf933+9WvTPd309eVp6AKFr9+fz+5Vh7puq5IDan+ehh2nnGIawPzFQ==",
	)
}

func TestHTTPFetcherFetchBlobSuccessSha256tree(t *testing.T) {
	testHTTPFetcherFetchBlobSuccessWithHasher(
		t,
		remoteexecution.DigestFunction_SHA256TREE,
		"35b974ff55d4c41ca000ea35b974ff55d4c41ca000eacf29125544cf29125544",
		"sha256tree-Nbl0/1XUxBygAOo1uXT/VdTEHKAA6s8pElVEzykSVUQ=",
	)
}

func testHTTPFetcherFetchBlobSuccessWithHasher(t *testing.T, digestFunctionEnum remoteexecution.DigestFunction_Value, hexHash, sriChecksum string) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{"www.example.com"},
		Qualifiers: []*remoteasset.Qualifier{
			{
				Name:  "checksum.sri",
				Value: sriChecksum,
			},
		},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)
	body := mock.NewMockReadCloser(ctrl)
	helloDigest := bb_digest.MustNewDigest(
		"",
		digestFunctionEnum,
		hexHash,
		5,
	)

	t.Run("Success"+helloDigest.GetDigestFunction().GetEnumValue().String(), func(t *testing.T) {
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
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

func TestHTTPFetcherFetchBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "www.example.com"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
		Qualifiers: []*remoteasset.Qualifier{
			{
				Name:  "checksum.sri",
				Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
			},
		},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)
	body := mock.NewMockReadCloser(ctrl)
	helloDigest := bb_digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969",
		5,
	)

	t.Run("SuccessNoExpectedDigest", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoExpectedDigestOrContentLength", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("UnknownChecksumSriAlgo", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
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
			InstanceName: "",
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
			InstanceName: "",
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
			InstanceName: "",
			Uris:         []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "bazel.auth_headers",
					Value: `{ "www.example.com": {"Authorization": "Bearer letmein"}}`,
				},
				{
					Name:  "checksum.sri",
					Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
				},
			},
		}
		matcher := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer letmein",
			},
		}
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
					Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
				},
			},
		}
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
