package fetch

import (
	"context"
	"regexp"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	config "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type policyRule struct {
	regex  *regexp.Regexp
	action config.FetchPolicy_Action
}

type policyFetcher struct {
	fetcher Fetcher
	rules   []policyRule
}

// NewPolicyFetcher creates a new Fetcher that applies a fetch policy.
func NewPolicyFetcher(fetcher Fetcher, cfg *config.FetchPolicy) (Fetcher, error) {
	rules := make([]policyRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		if r.Action == config.FetchPolicy_ACTION_UNSPECIFIED {
			return nil, status.Errorf(codes.InvalidArgument, "fetch policy rule for %q must specify an action", r.UriRegex)
		}
		re, err := regexp.Compile(r.UriRegex)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid URI regex %q: %v", r.UriRegex, err)
		}
		rules = append(rules, policyRule{
			regex:  re,
			action: r.Action,
		})
	}
	return &policyFetcher{
		fetcher: fetcher,
		rules:   rules,
	}, nil
}

func (pf *policyFetcher) evaluate(uri string) config.FetchPolicy_Action {
	for _, rule := range pf.rules {
		if rule.regex.MatchString(uri) {
			return rule.action
		}
	}
	return config.FetchPolicy_ACCEPT // Default to ACCEPT if no rules match
}

func (pf *policyFetcher) partitionURIs(uris []string) ([]string, []string) {
	var acceptURIs []string
	var refreshURIs []string

	for _, uri := range uris {
		action := pf.evaluate(uri)
		switch action {
		case config.FetchPolicy_DENY:
			// Filtered out
		case config.FetchPolicy_ACCEPT_REFRESH:
			refreshURIs = append(refreshURIs, uri)
		case config.FetchPolicy_ACCEPT:
			acceptURIs = append(acceptURIs, uri)
		}
	}
	return acceptURIs, refreshURIs
}

func (pf *policyFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	acceptURIs, refreshURIs := pf.partitionURIs(req.Uris)

	if len(acceptURIs) == 0 && len(refreshURIs) == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "Access denied by fetch policy")
	}

	var lastErr error

	// Try ACCEPT URIs first (normal cache behavior)
	if len(acceptURIs) > 0 {
		clone := *req
		clone.Uris = acceptURIs
		resp, err := pf.fetcher.FetchBlob(ctx, &clone)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	// Try ACCEPT_REFRESH URIs next (force refresh with OldestContentAccepted)
	if len(refreshURIs) > 0 {
		clone := *req
		clone.Uris = refreshURIs
		clone.OldestContentAccepted = timestamppb.New(time.Now().Add(time.Hour))

		resp, err := pf.fetcher.FetchBlob(ctx, &clone)
		if err == nil {
			return resp, nil
		}
		if lastErr != nil {
			return nil, status.Errorf(status.Code(err), "%s (normal fetch failed with: %v)", status.Convert(err).Message(), lastErr)
		}
		return nil, err
	}

	return nil, lastErr
}

func (pf *policyFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	acceptURIs, refreshURIs := pf.partitionURIs(req.Uris)

	if len(acceptURIs) == 0 && len(refreshURIs) == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "Access denied by fetch policy")
	}

	var lastErr error

	// Try ACCEPT URIs first (normal cache behavior)
	if len(acceptURIs) > 0 {
		clone := *req
		clone.Uris = acceptURIs
		resp, err := pf.fetcher.FetchDirectory(ctx, &clone)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	// Try ACCEPT_REFRESH URIs next (force refresh with OldestContentAccepted)
	if len(refreshURIs) > 0 {
		clone := *req
		clone.Uris = refreshURIs
		clone.OldestContentAccepted = timestamppb.New(time.Now().Add(time.Hour))

		resp, err := pf.fetcher.FetchDirectory(ctx, &clone)
		if err == nil {
			return resp, nil
		}
		if lastErr != nil {
			return nil, status.Errorf(status.Code(err), "%s (normal fetch failed with: %v)", status.Convert(err).Message(), lastErr)
		}
		return nil, err
	}

	return nil, lastErr
}

func (pf *policyFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return pf.fetcher.CheckQualifiers(qualifiers)
}
