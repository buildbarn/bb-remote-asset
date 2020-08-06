package storage_test

import (
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestAssetReferenceCreation(t *testing.T) {
	qualifiers := []*remoteasset.Qualifier{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "bar",
			Value: "foo",
		},
		{
			Name:  "foo",
			Value: "bap",
		},
	}

	sortedQualifiers := []*remoteasset.Qualifier{
		{
			Name:  "bar",
			Value: "foo",
		},
		{
			Name:  "foo",
			Value: "bap",
		},
		{
			Name:  "foo",
			Value: "bar",
		},
	}

	assetRef := storage.NewAssetReference("uri", qualifiers)
	require.Equal(t, sortedQualifiers, assetRef.Qualifiers)
	sortedRef := storage.NewAssetReference("uri", sortedQualifiers)
	require.Equal(t, sortedRef, assetRef)
}
