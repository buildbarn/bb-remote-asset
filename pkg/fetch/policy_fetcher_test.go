package fetch_test

import (
	"context"
	"testing"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	config "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/fetch"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPolicyFetcherNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	baseFetcher := mock.NewMockFetcher(ctrl)

	t.Run("InvalidRegex", func(t *testing.T) {
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: "[invalid",
					Action:   config.FetchPolicy_ACCEPT,
				},
			},
		}
		_, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("UnspecifiedAction", func(t *testing.T) {
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*",
					Action:   config.FetchPolicy_ACTION_UNSPECIFIED,
				},
			},
		}
		_, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestPolicyFetcherFetchBlob(t *testing.T) {
	ctrl := gomock.NewController(t)

	wantResp := &remoteasset.FetchBlobResponse{
		Status: status.New(codes.OK, "Success").Proto(),
	}

	t.Run("DefaultAcceptAll", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{} // Empty policy
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/foo", "http://example.com/bar"},
		}

		baseFetcher.EXPECT().FetchBlob(ctx, req).Return(wantResp, nil)

		resp, err := pf.FetchBlob(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})

	t.Run("DenyAll", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*",
					Action:   config.FetchPolicy_DENY,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/foo"},
		}

		_, err = pf.FetchBlob(ctx, req)
		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("FilterDeniedURIs", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*deny.*",
					Action:   config.FetchPolicy_DENY,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/deny-this", "http://example.com/allow-this"},
		}

		expectedReq := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/allow-this"},
		}

		baseFetcher.EXPECT().FetchBlob(ctx, expectedReq).Return(wantResp, nil)

		resp, err := pf.FetchBlob(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})

	t.Run("AcceptRefreshForcesBypass", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*refresh.*",
					Action:   config.FetchPolicy_ACCEPT_REFRESH,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/refresh-this", "http://example.com/normal"},
		}

		// 1. Expect call for ACCEPT bucket ("normal")
		reqNormal := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/normal"},
		}
		// Mock it to fail so we proceed to refresh bucket
		baseFetcher.EXPECT().FetchBlob(ctx, reqNormal).Return(nil, status.Error(codes.NotFound, "not cached"))

		// 2. Expect call for REFRESH bucket ("refresh-this") with OldestContentAccepted set
		baseFetcher.EXPECT().FetchBlob(ctx, gomock.Any()).DoAndReturn(
			func(ctx context.Context, r *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
				require.Equal(t, []string{"http://example.com/refresh-this"}, r.Uris)
				require.NotNil(t, r.OldestContentAccepted)
				require.True(t, r.OldestContentAccepted.AsTime().After(time.Now()))
				return wantResp, nil
			},
		)

		resp, err := pf.FetchBlob(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})

	t.Run("AcceptBucketSucceedsFirst", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*refresh.*",
					Action:   config.FetchPolicy_ACCEPT_REFRESH,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/refresh-this", "http://example.com/normal"},
		}

		// Expect call for ACCEPT bucket ("normal")
		reqNormal := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/normal"},
		}
		// Mock it to succeed
		baseFetcher.EXPECT().FetchBlob(ctx, reqNormal).Return(wantResp, nil)

		// We expect NO call for REFRESH bucket

		resp, err := pf.FetchBlob(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})

	t.Run("FirstMatchWins", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*special.*",
					Action:   config.FetchPolicy_DENY,
				},
				{
					UriRegex: ".*",
					Action:   config.FetchPolicy_ACCEPT,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/special", "http://example.com/normal"},
		}

		expectedReq := &remoteasset.FetchBlobRequest{
			Uris: []string{"http://example.com/normal"},
		}

		baseFetcher.EXPECT().FetchBlob(ctx, expectedReq).Return(wantResp, nil)

		resp, err := pf.FetchBlob(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})
}

func TestPolicyFetcherFetchDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)

	wantResp := &remoteasset.FetchDirectoryResponse{
		Status: status.New(codes.OK, "Success").Proto(),
	}

	t.Run("FilterDeniedURIs", func(t *testing.T) {
		ctx := context.Background()
		baseFetcher := mock.NewMockFetcher(ctrl)
		cfg := &config.FetchPolicy{
			Rules: []*config.FetchPolicy_Rule{
				{
					UriRegex: ".*deny.*",
					Action:   config.FetchPolicy_DENY,
				},
			},
		}
		pf, err := fetch.NewPolicyFetcher(baseFetcher, cfg)
		require.NoError(t, err)

		req := &remoteasset.FetchDirectoryRequest{
			Uris: []string{"http://example.com/deny-this", "http://example.com/allow-this"},
		}

		expectedReq := &remoteasset.FetchDirectoryRequest{
			Uris: []string{"http://example.com/allow-this"},
		}

		baseFetcher.EXPECT().FetchDirectory(ctx, expectedReq).Return(wantResp, nil)

		resp, err := pf.FetchDirectory(ctx, req)
		require.NoError(t, err)
		require.Equal(t, wantResp, resp)
	})
}
