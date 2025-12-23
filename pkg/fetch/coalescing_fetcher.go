package fetch

import (
	"context"
	"sort"
	"strings"
	"sync"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
)

// coalescingFetcher wraps a Fetcher to deduplicate concurrent requests for the
// same resource. When multiple requests arrive for the same URI(s) and qualifiers
// before the first request completes, subsequent requests wait for the first
// request's result rather than triggering duplicate upstream fetches.
//
// This is particularly useful for the HTTP fetcher path where there is no
// external deduplication mechanism (unlike the remote execution path which
// benefits from scheduler-level deduplication).
type coalescingFetcher struct {
	fetcher      Fetcher
	mu           sync.Mutex
	inFlightBlob map[string]*blobFetchOperation
	inFlightDir  map[string]*dirFetchOperation
}

type blobFetchOperation struct {
	done   chan struct{}
	result *remoteasset.FetchBlobResponse
	err    error
}

type dirFetchOperation struct {
	done   chan struct{}
	result *remoteasset.FetchDirectoryResponse
	err    error
}

// NewCoalescingFetcher creates a decorator that deduplicates concurrent fetch
// requests for the same resource. Requests are considered identical if they
// have the same URIs and stable qualifiers (excluding volatile qualifiers like
// authentication headers).
func NewCoalescingFetcher(fetcher Fetcher) Fetcher {
	return &coalescingFetcher{
		fetcher:      fetcher,
		inFlightBlob: make(map[string]*blobFetchOperation),
		inFlightDir:  make(map[string]*dirFetchOperation),
	}
}

func (cf *coalescingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	// Build coalescing key using the same stable qualifiers as caching
	key := buildCoalescingKey("blob", req.Uris, RemoveVolatileQualifiers(req.Qualifiers))

	cf.mu.Lock()
	if op, ok := cf.inFlightBlob[key]; ok {
		cf.mu.Unlock()
		// Wait for in-flight operation to complete
		select {
		case <-op.done:
			return op.result, op.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// First caller for this key - create operation and register it
	op := &blobFetchOperation{done: make(chan struct{})}
	cf.inFlightBlob[key] = op
	cf.mu.Unlock()

	// Perform the actual fetch
	op.result, op.err = cf.fetcher.FetchBlob(ctx, req)

	// Cleanup and notify waiters
	cf.mu.Lock()
	delete(cf.inFlightBlob, key)
	cf.mu.Unlock()
	close(op.done)

	return op.result, op.err
}

func (cf *coalescingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	// Build coalescing key using the same stable qualifiers as caching
	key := buildCoalescingKey("dir", req.Uris, RemoveVolatileQualifiers(req.Qualifiers))

	cf.mu.Lock()
	if op, ok := cf.inFlightDir[key]; ok {
		cf.mu.Unlock()
		// Wait for in-flight operation to complete
		select {
		case <-op.done:
			return op.result, op.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// First caller for this key - create operation and register it
	op := &dirFetchOperation{done: make(chan struct{})}
	cf.inFlightDir[key] = op
	cf.mu.Unlock()

	// Perform the actual fetch
	op.result, op.err = cf.fetcher.FetchDirectory(ctx, req)

	// Cleanup and notify waiters
	cf.mu.Lock()
	delete(cf.inFlightDir, key)
	cf.mu.Unlock()
	close(op.done)

	return op.result, op.err
}

func (cf *coalescingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return cf.fetcher.CheckQualifiers(qualifiers)
}

// buildCoalescingKey creates a stable string key from the operation type, URIs,
// and qualifiers. The key is used to identify identical requests that can share
// a single upstream fetch. Volatile qualifiers (auth headers) should be removed
// via RemoveVolatileQualifiers before calling this function.
func buildCoalescingKey(opType string, uris []string, qualifiers []*remoteasset.Qualifier) string {
	var b strings.Builder

	// Include operation type prefix
	b.WriteString(opType)
	b.WriteString("|")

	// Sort and join URIs for stable ordering
	sortedURIs := make([]string, len(uris))
	copy(sortedURIs, uris)
	sort.Strings(sortedURIs)
	b.WriteString(strings.Join(sortedURIs, ","))
	b.WriteString("|")

	// Sort qualifiers by name for stable ordering
	sortedQuals := make([]*remoteasset.Qualifier, len(qualifiers))
	copy(sortedQuals, qualifiers)
	sort.Slice(sortedQuals, func(i, j int) bool {
		return sortedQuals[i].Name < sortedQuals[j].Name
	})

	// Append qualifier name=value pairs
	for i, q := range sortedQuals {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(q.Name)
		b.WriteString("=")
		b.WriteString(q.Value)
	}

	return b.String()
}
