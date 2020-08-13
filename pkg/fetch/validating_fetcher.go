package fetch

import (
	"context"
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/qualifier"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validatingFetcher struct {
	fetcher Fetcher
}

// NewValidatingFetcher creates a fetcher that validates Fetch* requests are valid,
// before passing on to a backend
func NewValidatingFetcher(fetcher Fetcher) Fetcher {
	return &validatingFetcher{
		fetcher: fetcher,
	}
}

func (vf *validatingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "FetchBlob does not support requests without any URIs specified.")
	}
	if unsupported := vf.CheckQualifiers(qualifier.QualifiersToSet(req.Qualifiers)); !(unsupported.IsEmpty()) {
		violations := []*errdetails.BadRequest_FieldViolation{}
		for q := range unsupported {
			violations = append(violations, &errdetails.BadRequest_FieldViolation{
				Field:       "qualifiers.name",
				Description: fmt.Sprintf("\"%s\" not supported", q),
			})
		}
		s, err := status.New(codes.InvalidArgument, "Unsupported Qualifier(s) found in request.").WithDetails(
			&errdetails.BadRequest{
				FieldViolations: violations,
			})
		if err != nil {
			return nil, err
		}
		return nil, s.Err()
	}
	return vf.fetcher.FetchBlob(ctx, req)
}

func (vf *validatingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "FetchDirectory does not support requests without any URIs specified.")
	}
	if unsupported := vf.CheckQualifiers(qualifier.QualifiersToSet(req.Qualifiers)); !(unsupported.IsEmpty()) {
		violations := []*errdetails.BadRequest_FieldViolation{}
		for q := range unsupported {
			violations = append(violations, &errdetails.BadRequest_FieldViolation{
				Field:       "qualifiers.name",
				Description: fmt.Sprintf("\"%s\" not supported", q),
			})
		}
		s, err := status.New(codes.InvalidArgument, "Unsupported Qualifier(s) found in request.").WithDetails(
			&errdetails.BadRequest{
				FieldViolations: violations,
			})
		if err != nil {
			return nil, err
		}
		return nil, s.Err()
	}
	return vf.fetcher.FetchDirectory(ctx, req)
}

func (vf *validatingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return vf.fetcher.CheckQualifiers(qualifiers)
}
