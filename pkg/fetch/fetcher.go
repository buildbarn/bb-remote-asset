package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/qualifier"
)

// Fetcher is an abstraction around a Remote Asset API Fetch Server to allow for more consistent
// Qualifier usage.
type Fetcher interface {
	// The same as a Remote Asset API FetchBlob request
	FetchBlob(context.Context, *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error)

	// The same as a Remote Asset API FetchDirectory request
	FetchDirectory(context.Context, *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error)

	// Check for unsupported Qualifiers, returning a set of the _unsupported_ qualifiers
	CheckQualifiers(qualifier.Set) qualifier.Set
}
