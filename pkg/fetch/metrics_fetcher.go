package fetch

import (
	"context"
	"sync"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
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
			Buckets:   util.DecimalExponentialBuckets(-3, 6, 2),
		},
		[]string{"name", "operation"})
	blobAccessOperationsDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "buildbarn",
			Subsystem: "remote_asset",
			Name:      "http_fetcher_duration_seconds",
			Help:      "Amount of time spent per operation on fetching remote assets, in seconds.",
			Buckets:   util.DecimalExponentialBuckets(-3, 6, 2),
		},
		[]string{"name", "operation", "status"})
)

type metricsFetcher struct {
	fetcher remoteasset.FetchServer
	clock   clock.Clock

	fetchBlobBlobSizeBytes        prometheus.Observer
	fetchBlobDurationSeconds      prometheus.ObserverVec
	fetchDirectoryDurationSeconds prometheus.ObserverVec
}

// NewMetricsFetcher creates a fetcher which logs metrics to prometheus
func NewMetricsFetcher(fetcher remoteasset.FetchServer, clock clock.Clock, name string) remoteasset.FetchServer {
	httpFetcherOperationsPrometheusMetrics.Do(func() {
		prometheus.MustRegister(httpFetcherOperationsBlobSizeBytes)
		prometheus.MustRegister(blobAccessOperationsDurationSeconds)
	})

	return &metricsFetcher{
		fetcher: fetcher,
		clock:   clock,

		fetchBlobBlobSizeBytes:        httpFetcherOperationsBlobSizeBytes.WithLabelValues(name, "Fetch Blob"),
		fetchBlobDurationSeconds:      blobAccessOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "FetchBlob"}),
		fetchDirectoryDurationSeconds: blobAccessOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "FetchDirectory"}),
	}
}

func (mf *metricsFetcher) updateDurationSeconds(vec prometheus.ObserverVec, code codes.Code, timeStart time.Time) {
	vec.WithLabelValues(code.String()).Observe(mf.clock.Now().Sub(timeStart).Seconds())
}

func (mf *metricsFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	timeStart := mf.clock.Now()
	resp, err := mf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		mf.updateDurationSeconds(mf.fetchBlobDurationSeconds, status.Code(err), timeStart)
		return nil, err
	}
	mf.updateDurationSeconds(mf.fetchBlobDurationSeconds, codes.Code(resp.Status.Code), timeStart)
	mf.fetchBlobBlobSizeBytes.Observe(float64(resp.BlobDigest.SizeBytes))
	return resp, err
}

func (mf *metricsFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	timeStart := mf.clock.Now()
	resp, err := mf.fetcher.FetchDirectory(ctx, req)
	if err != nil {
		mf.updateDurationSeconds(mf.fetchDirectoryDurationSeconds, status.Code(err), timeStart)
		return nil, err
	}
	mf.updateDurationSeconds(mf.fetchDirectoryDurationSeconds, codes.Code(resp.Status.Code), timeStart)
	return resp, err
}
