package storage_test

import (
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestAssetReferenceCreation(t *testing.T) {
	qualifiers := []*remoteasset.Qualifier{
		&remoteasset.Qualifier{
			Name:  "foo",
			Value: "bar",
		},
		&remoteasset.Qualifier{
			Name:  "bar",
			Value: "foo",
		},
		&remoteasset.Qualifier{
			Name:  "foo",
			Value: "bap",
		},
	}

	sortedQualifiers := []*remoteasset.Qualifier{
		&remoteasset.Qualifier{
			Name:  "bar",
			Value: "foo",
		},
		&remoteasset.Qualifier{
			Name:  "foo",
			Value: "bap",
		},
		&remoteasset.Qualifier{
			Name:  "foo",
			Value: "bar",
		},
	}

	assetRef := storage.NewAssetReference("uri", qualifiers)
	require.Equal(t, sortedQualifiers, assetRef.Qualifiers)
	sortedRef := storage.NewAssetReference("uri", sortedQualifiers)
	require.Equal(t, sortedRef, assetRef)
}
