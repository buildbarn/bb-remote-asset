package fetch

import (
	"context"
	"sync"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/clock"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	httpFetcherOperationsPrometheusMetrics sync.Once

	httpFetcherOperationsBlobSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "buildbarn",
			Subsystem: "remote_asset",
			Name:      "http_fetcher_blob_size_bytes",
			Help:      "Size of blobs fetched using the http fetcher, in bytes",
			Buckets:   util.DecimalExponentialBuckets(1, 6, 2),
		},
		[]string{"name", "operation", "resource_type"})
	blobAccessOperationsDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "buildbarn",
			Subsystem: "remote_asset",
			Name:      "http_fetcher_duration_seconds",
			Help:      "Amount of time spent per operation on fetching remote assets, in seconds.",
			Buckets:   util.DecimalExponentialBuckets(-3, 6, 2),
		},
		[]string{"name", "operation", "status", "resource_type"})
)

type metricsFetcher struct {
	fetcher Fetcher
	clock   clock.Clock

	fetchBlobBlobSizeBytes        prometheus.ObserverVec
	fetchBlobDurationSeconds      prometheus.ObserverVec
	fetchDirectoryDurationSeconds prometheus.ObserverVec
}

// NewMetricsFetcher creates a fetcher which logs metrics to prometheus
func NewMetricsFetcher(fetcher Fetcher, clock clock.Clock, name string) Fetcher {
	httpFetcherOperationsPrometheusMetrics.Do(func() {
		prometheus.MustRegister(httpFetcherOperationsBlobSizeBytes)
		prometheus.MustRegister(blobAccessOperationsDurationSeconds)
	})

	return &metricsFetcher{
		fetcher: fetcher,
		clock:   clock,

		fetchBlobBlobSizeBytes:        httpFetcherOperationsBlobSizeBytes.MustCurryWith(map[string]string{"name": name, "operation": "Fetch Blob"}),
		fetchBlobDurationSeconds:      blobAccessOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "FetchBlob"}),
		fetchDirectoryDurationSeconds: blobAccessOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "FetchDirectory"}),
	}
}

func (mf *metricsFetcher) updateDurationSeconds(vec prometheus.ObserverVec, code codes.Code, timeStart time.Time, qualifiers []*remoteasset.Qualifier) {
	if len(qualifiers) == 0 {
		vec.WithLabelValues(code.String(), "N/A").Observe(mf.clock.Now().Sub(timeStart).Seconds())
	} else {
		resourceType := "N/A"
		for _, qualifier := range qualifiers {
			if qualifier.Name == "resource_type" {
				resourceType = qualifier.Value
				break
			}
		}
		vec.WithLabelValues(code.String(), resourceType).Observe(mf.clock.Now().Sub(timeStart).Seconds())
	}
}

func (mf *metricsFetcher) updateBlobSizeBytes(vec prometheus.ObserverVec, blobSize float64, qualifiers []*remoteasset.Qualifier) {
	if len(qualifiers) == 0 {
		vec.WithLabelValues("N/A").Observe(blobSize)
	} else {
		resourceType := "N/A"
		for _, qualifier := range qualifiers {
			if qualifier.Name == "resource_type" {
				resourceType = qualifier.Value
				break
			}
		}
		vec.WithLabelValues(resourceType).Observe(blobSize)
	}
}

func (mf *metricsFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	timeStart := mf.clock.Now()
	resp, err := mf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		mf.updateDurationSeconds(mf.fetchBlobDurationSeconds, status.Code(err), timeStart, req.Qualifiers)
		return nil, err
	}
	mf.updateDurationSeconds(mf.fetchBlobDurationSeconds, codes.Code(resp.Status.Code), timeStart, req.Qualifiers)
	mf.updateBlobSizeBytes(mf.fetchBlobBlobSizeBytes, float64(resp.BlobDigest.SizeBytes), req.Qualifiers)
	return resp, err
}

func (mf *metricsFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	timeStart := mf.clock.Now()
	resp, err := mf.fetcher.FetchDirectory(ctx, req)
	if err != nil {
		mf.updateDurationSeconds(mf.fetchDirectoryDurationSeconds, status.Code(err), timeStart, req.Qualifiers)
		return nil, err
	}
	mf.updateDurationSeconds(mf.fetchDirectoryDurationSeconds, codes.Code(resp.Status.Code), timeStart, req.Qualifiers)
	return resp, err
}

func (mf *metricsFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return mf.fetcher.CheckQualifiers(qualifiers)
}
