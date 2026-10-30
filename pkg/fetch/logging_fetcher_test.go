package fetch

import (
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/stretchr/testify/require"
)

func newTestLoggingFetcher(loggedHeaderNames []string) *loggingFetcher {
	lf := NewLoggingFetcher(nil, loggedHeaderNames)
	return lf.(*loggingFetcher)
}

func TestIsHeaderNameLogged(t *testing.T) {
	lf := newTestLoggingFetcher([]string{"Authorization", "X-Custom"})

	require.True(t, lf.isHeaderNameLogged("Authorization"))
	require.True(t, lf.isHeaderNameLogged("authorization"))
	require.True(t, lf.isHeaderNameLogged("X-Custom"))
	require.False(t, lf.isHeaderNameLogged("X-Other"))
}

func TestRedactQualifierValue(t *testing.T) {
	lf := newTestLoggingFetcher([]string{"X-Custom"})

	tests := []struct {
		name string
		q    *remoteasset.Qualifier
		want string
	}{
		{
			name: "http_header allowed",
			q:    &remoteasset.Qualifier{Name: "http_header:X-Custom", Value: "visible-value"},
			want: "visible-value",
		},
		{
			name: "http_header not allowed",
			q:    &remoteasset.Qualifier{Name: "http_header:Authorization", Value: "Bearer secret-token"},
			want: redactedValue,
		},
		{
			name: "http_header_url allowed",
			q:    &remoteasset.Qualifier{Name: "http_header_url:0:X-Custom", Value: "visible-value"},
			want: "visible-value",
		},
		{
			name: "http_header_url not allowed",
			q:    &remoteasset.Qualifier{Name: "http_header_url:0:Authorization", Value: "Bearer secret-token"},
			want: redactedValue,
		},
		{
			name: "http_header_url malformed",
			q:    &remoteasset.Qualifier{Name: "http_header_url:not-an-index:X-Custom", Value: "visible-value"},
			want: redactedValue,
		},
		{
			name: "legacy auth headers",
			q:    &remoteasset.Qualifier{Name: QualifierLegacyBazelHTTPHeaders, Value: `{"source.test":{"X-Custom":"visible-value"}}`},
			want: `{"source.test":{"X-Custom":"visible-value"}}`,
		},
		{
			name: "legacy auth headers malformed",
			q:    &remoteasset.Qualifier{Name: QualifierLegacyBazelHTTPHeaders, Value: "not-json"},
			want: redactedValue,
		},
		{
			name: "other qualifier passthrough",
			q:    &remoteasset.Qualifier{Name: "checksum.sri", Value: "sha256-abc123"},
			want: "sha256-abc123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, lf.redactQualifierValue(tc.q))
		})
	}
}

func TestRedactLegacyAuthHeaders(t *testing.T) {
	lf := newTestLoggingFetcher([]string{"X-Custom"})

	t.Run("mixed allowed and redacted headers", func(t *testing.T) {
		got := lf.redactLegacyAuthHeaders(`{"source.test":{"Authorization":"Bearer secret-token","X-Custom":"visible-value"}}`)
		require.NotContains(t, got, "secret-token")
		require.Contains(t, got, "visible-value")

		ah, err := NewAuthHeadersFromQualifier(got)
		require.NoError(t, err)
		require.Equal(t, redactedValue, (*ah)["source.test"]["Authorization"])
		require.Equal(t, "visible-value", (*ah)["source.test"]["X-Custom"])
	})

	t.Run("malformed JSON", func(t *testing.T) {
		require.Equal(t, redactedValue, lf.redactLegacyAuthHeaders("not-json"))
	})
}

func TestRedactQualifiers(t *testing.T) {
	lf := newTestLoggingFetcher([]string{"X-Custom"})

	qualifiers := []*remoteasset.Qualifier{
		{Name: "http_header:X-Custom", Value: "visible-value"},
		{Name: "http_header:Authorization", Value: "Bearer secret-token"},
		{Name: "checksum.sri", Value: "sha256-abc123"},
	}

	got := lf.redactQualifiers(qualifiers)
	require.NotContains(t, got, "secret-token")
	require.Contains(t, got, `name:"http_header:X-Custom" value:"visible-value"`)
	require.Contains(t, got, `name:"http_header:Authorization" value:"<redacted>"`)
	require.Contains(t, got, `name:"checksum.sri" value:"sha256-abc123"`)
}
