package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"google.golang.org/grpc/status"
)

const redactedValue = "<redacted>"

type loggingFetcher struct {
	fetcher           Fetcher
	loggedHeaderNames map[string]struct{}
}

// NewLoggingFetcher creates a fetcher which logs requests and results.
//
// Qualifier values that carry HTTP header contents which may contain secrets such as "Authorization" tokens.
// Their values are redacted from the log unless the corresponding header name is present in loggedHeaderNames.
func NewLoggingFetcher(fetcher Fetcher, loggedHeaderNames []string) Fetcher {
	names := make(map[string]struct{}, len(loggedHeaderNames))
	for _, name := range loggedHeaderNames {
		names[strings.ToLower(name)] = struct{}{}
	}
	return &loggingFetcher{
		fetcher:           fetcher,
		loggedHeaderNames: names,
	}
}

func (lf *loggingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	log.Printf("Fetching Blob %s with qualifiers %s", req.Uris, lf.redactQualifiers(req.Qualifiers))
	resp, err := lf.fetcher.FetchBlob(ctx, req)
	if err == nil {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	} else {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}

func (lf *loggingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	log.Printf("Fetching Directory %s with qualifiers %s", req.Uris, lf.redactQualifiers(req.Qualifiers))
	resp, err := lf.fetcher.FetchDirectory(ctx, req)
	if err == nil {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	} else {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}

func (lf *loggingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return lf.fetcher.CheckQualifiers(qualifiers)
}

func (lf *loggingFetcher) isHeaderNameLogged(headerName string) bool {
	_, ok := lf.loggedHeaderNames[strings.ToLower(headerName)]
	return ok
}

// redactQualifiers formats qualifiers for logging,
// redacting the values of any HTTP header qualifiers whose header name isn't whitelisted.
func (lf *loggingFetcher) redactQualifiers(qualifiers []*remoteasset.Qualifier) string {
	parts := make([]string, 0, len(qualifiers))
	for _, q := range qualifiers {
		parts = append(parts, fmt.Sprintf("name:%q value:%q", q.Name, lf.redactQualifierValue(q)))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func (lf *loggingFetcher) redactQualifierValue(q *remoteasset.Qualifier) string {
	switch {
	case q.Name == QualifierLegacyBazelHTTPHeaders:
		return lf.redactLegacyAuthHeaders(q.Value)
	case strings.HasPrefix(q.Name, QualifierHTTPHeaderURLPrefix):
		_, header, err := parseHTTPHeaderURLQualifierName(q.Name)
		if err != nil || !lf.isHeaderNameLogged(header) {
			return redactedValue
		}
		return q.Value
	case strings.HasPrefix(q.Name, QualifierHTTPHeaderPrefix):
		header := strings.TrimPrefix(q.Name, QualifierHTTPHeaderPrefix)
		if !lf.isHeaderNameLogged(header) {
			return redactedValue
		}
		return q.Value
	default:
		return q.Value
	}
}

// redactLegacyAuthHeaders redacts the header values carried by a legacy
// "bazel.auth_headers" qualifier, keeping only whitelisted header names.
func (lf *loggingFetcher) redactLegacyAuthHeaders(value string) string {
	ah, err := NewAuthHeadersFromQualifier(value)
	if err != nil {
		return redactedValue
	}
	redacted := NewAuthHeaders()
	for uri, headers := range *ah {
		for header, v := range headers {
			if lf.isHeaderNameLogged(header) {
				redacted.AddHeader(uri, header, v)
			} else {
				redacted.AddHeader(uri, header, redactedValue)
			}
		}
	}
	b, err := json.Marshal(redacted)
	if err != nil {
		return redactedValue
	}
	return string(b)
}
