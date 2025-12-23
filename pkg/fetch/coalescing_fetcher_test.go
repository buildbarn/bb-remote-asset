package fetch_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCoalescingFetcherBlobDeduplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test.tar.gz"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	blobDigest := &remoteexecution.Digest{
		Hash:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SizeBytes: 200,
	}

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	// Mock fetcher with delay and call counter
	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond) // Ensure all goroutines have time to queue
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "fetched").Proto(),
				Uri:        uri,
				BlobDigest: blobDigest,
			}, nil
		}).
		AnyTimes()

	// Launch concurrent requests
	const numGoroutines = 10
	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	responses := make([]*remoteasset.FetchBlobResponse, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startBarrier.Wait()
			responses[idx], errors[idx] = coalescingFetcher.FetchBlob(ctx, request)
		}(i)
	}

	startBarrier.Done()
	wg.Wait()

	// With coalescing, only 1 fetch should occur
	require.Equal(t, int32(1), fetchCount.Load(), "Expected exactly 1 fetch call with coalescing")

	// All responses should be successful
	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errors[i], "Request %d should not have error", i)
		require.NotNil(t, responses[i], "Request %d should have response", i)
		require.Equal(t, int32(codes.OK), responses[i].Status.Code)
	}
}

func TestCoalescingFetcherDirectoryDeduplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test-dir.zip"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	dirDigest := &remoteexecution.Digest{
		Hash:      "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		SizeBytes: 300,
	}

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	mockFetcher.EXPECT().FetchDirectory(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return &remoteasset.FetchDirectoryResponse{
				Status:              status.New(codes.OK, "fetched").Proto(),
				Uri:                 uri,
				RootDirectoryDigest: dirDigest,
			}, nil
		}).
		AnyTimes()

	const numGoroutines = 10
	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startBarrier.Wait()
			_, _ = coalescingFetcher.FetchDirectory(ctx, request)
		}()
	}

	startBarrier.Done()
	wg.Wait()

	require.Equal(t, int32(1), fetchCount.Load(), "Expected exactly 1 fetch call with coalescing")
}

func TestCoalescingFetcherDifferentURIsNotCoalesced(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "fetched").Proto(),
				Uri:        req.Uris[0],
				BlobDigest: &remoteexecution.Digest{Hash: "aaaa", SizeBytes: 1},
			}, nil
		}).
		AnyTimes()

	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	// Launch requests for 3 different URIs
	uris := []string{
		"https://example.com/file1.tar.gz",
		"https://example.com/file2.tar.gz",
		"https://example.com/file3.tar.gz",
	}

	for _, uri := range uris {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			startBarrier.Wait()
			_, _ = coalescingFetcher.FetchBlob(ctx, &remoteasset.FetchBlobRequest{
				Uris: []string{u},
			})
		}(uri)
	}

	startBarrier.Done()
	wg.Wait()

	// Each different URI should trigger its own fetch
	require.Equal(t, int32(3), fetchCount.Load(), "Different URIs should not be coalesced")
}

func TestCoalescingFetcherVolatileQualifiersIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test.tar.gz"

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "fetched").Proto(),
				Uri:        uri,
				BlobDigest: &remoteexecution.Digest{Hash: "aaaa", SizeBytes: 1},
			}, nil
		}).
		AnyTimes()

	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	// Launch requests with same URI but different auth headers
	requests := []*remoteasset.FetchBlobRequest{
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "bazel.auth_headers", Value: "token-1"},
			},
		},
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "bazel.auth_headers", Value: "token-2"},
			},
		},
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "http_header_url:0:Authorization", Value: "Bearer abc"},
			},
		},
	}

	for _, req := range requests {
		wg.Add(1)
		go func(r *remoteasset.FetchBlobRequest) {
			defer wg.Done()
			startBarrier.Wait()
			_, _ = coalescingFetcher.FetchBlob(ctx, r)
		}(req)
	}

	startBarrier.Done()
	wg.Wait()

	// Volatile qualifiers should be ignored, so all requests coalesce
	require.Equal(t, int32(1), fetchCount.Load(), "Requests differing only in volatile qualifiers should coalesce")
}

func TestCoalescingFetcherDifferentChecksumsNotCoalesced(t *testing.T) {
	// checksum.sri is treated as a stable qualifier (same as caching).
	// Requests with different checksums will not coalesce - this ensures
	// consistent behavior between caching and coalescing key generation.
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test.tar.gz"

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "fetched").Proto(),
				Uri:        uri,
				BlobDigest: &remoteexecution.Digest{Hash: "aaaa", SizeBytes: 1},
			}, nil
		}).
		AnyTimes()

	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	// Launch requests with same URI but different expected checksums
	requests := []*remoteasset.FetchBlobRequest{
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "checksum.sri", Value: "sha256-aaa"},
			},
		},
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "checksum.sri", Value: "sha256-bbb"},
			},
		},
	}

	for _, req := range requests {
		wg.Add(1)
		go func(r *remoteasset.FetchBlobRequest) {
			defer wg.Done()
			startBarrier.Wait()
			_, _ = coalescingFetcher.FetchBlob(ctx, r)
		}(req)
	}

	startBarrier.Done()
	wg.Wait()

	// Different checksums = different coalescing keys (consistent with caching)
	require.Equal(t, int32(2), fetchCount.Load(), "Requests with different checksum.sri should not coalesce")
}

func TestCoalescingFetcherDifferentResourceTypeNotCoalesced(t *testing.T) {
	// resource_type affects what the downstream fetcher does (e.g., extraction)
	// so requests with different resource types should NOT coalesce
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test.tar.gz"

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "fetched").Proto(),
				Uri:        uri,
				BlobDigest: &remoteexecution.Digest{Hash: "aaaa", SizeBytes: 1},
			}, nil
		}).
		AnyTimes()

	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	// Launch requests with same URI but different resource types
	requests := []*remoteasset.FetchBlobRequest{
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "resource_type", Value: "application/x-tar"},
			},
		},
		{
			Uris: []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{Name: "resource_type", Value: "application/zip"},
			},
		},
	}

	for _, req := range requests {
		wg.Add(1)
		go func(r *remoteasset.FetchBlobRequest) {
			defer wg.Done()
			startBarrier.Wait()
			_, _ = coalescingFetcher.FetchBlob(ctx, r)
		}(req)
	}

	startBarrier.Done()
	wg.Wait()

	// Different resource_type qualifiers should NOT coalesce
	require.Equal(t, int32(2), fetchCount.Load(), "Requests with different resource_type should not coalesce")
}

func TestCoalescingFetcherContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)

	uri := "https://example.com/test.tar.gz"
	request := &remoteasset.FetchBlobRequest{
		Uris: []string{uri},
	}

	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	// Mock fetcher that takes a long time
	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			time.Sleep(500 * time.Millisecond)
			return &remoteasset.FetchBlobResponse{
				Status: status.New(codes.OK, "fetched").Proto(),
			}, nil
		}).
		AnyTimes()

	// Start the first request that will actually do the fetch
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		_, _ = coalescingFetcher.FetchBlob(ctx, request)
	}()

	// Give the first request time to start
	time.Sleep(10 * time.Millisecond)

	// Second request with a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := coalescingFetcher.FetchBlob(ctx, request)
	require.Error(t, err)
	require.Equal(t, context.Canceled, err)

	wg.Wait()
}

func TestCoalescingFetcherErrorPropagation(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	uri := "https://example.com/test.tar.gz"
	request := &remoteasset.FetchBlobRequest{
		Uris: []string{uri},
	}

	var fetchCount atomic.Int32
	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	expectedErr := status.Error(codes.NotFound, "resource not found")

	mockFetcher.EXPECT().FetchBlob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
			fetchCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return nil, expectedErr
		}).
		AnyTimes()

	const numGoroutines = 5
	var wg sync.WaitGroup
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startBarrier.Wait()
			_, errors[idx] = coalescingFetcher.FetchBlob(ctx, request)
		}(i)
	}

	startBarrier.Done()
	wg.Wait()

	// Only 1 fetch should occur
	require.Equal(t, int32(1), fetchCount.Load())

	// All requests should receive the same error
	for i, err := range errors {
		require.Error(t, err, "Request %d should have error", i)
		require.Equal(t, codes.NotFound, status.Code(err), "Request %d should have NotFound error", i)
	}
}

func TestCoalescingFetcherCheckQualifiers(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockFetcher := mock.NewMockFetcher(ctrl)
	coalescingFetcher := fetch.NewCoalescingFetcher(mockFetcher)

	// CheckQualifiers should delegate to underlying fetcher
	mockFetcher.EXPECT().CheckQualifiers(gomock.Any()).Return(nil)
	result := coalescingFetcher.CheckQualifiers(nil)
	require.Nil(t, result)
}
