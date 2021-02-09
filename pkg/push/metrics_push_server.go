package push

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
	assetStoreOperationsPrometheusMetrics sync.Once

	pushServerOperationsBlobSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "buildbarn",
			Subsystem: "push_server",
			Name:      "push_server_blob_size_bytes",
			Help:      "Size of blobs being pushed, in bytes.",
			Buckets:   util.DecimalExponentialBuckets(1, 6, 2),
		},
		[]string{"name", "operation", "resource_type"})
	pushServerOperationsDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "buildbarn",
			Subsystem: "push_server",
			Name:      "blob_access_duration_seconds",
			Help:      "Amount of time spent per operation on pushing remote assets, in seconds.",
			Buckets:   util.DecimalExponentialBuckets(-3, 6, 2),
		},
		[]string{"name", "operation", "grpc_code", "resource_type"})

	// todo(arlyon): directory size?
)

type metricsAssetPushServer struct {
	pushServer remoteasset.PushServer
	clock      clock.Clock

	pushBlobBlobSizeBytes        prometheus.ObserverVec
	pushBlobDurationSeconds      prometheus.ObserverVec
	pushDirectoryDurationSeconds prometheus.ObserverVec
}

// NewMetricsAssetPushServer wraps the PushServer to
// report prometheus metrics.
func NewMetricsAssetPushServer(ps remoteasset.PushServer, clock clock.Clock, name string) remoteasset.PushServer {
	assetStoreOperationsPrometheusMetrics.Do(func() {
		prometheus.MustRegister(pushServerOperationsBlobSizeBytes)
		prometheus.MustRegister(pushServerOperationsDurationSeconds)
	})

	return &metricsAssetPushServer{
		pushServer: ps,
		clock:      clock,

		pushBlobBlobSizeBytes:        pushServerOperationsBlobSizeBytes.MustCurryWith(map[string]string{"name": name, "operation": "PushBlob"}),
		pushBlobDurationSeconds:      pushServerOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "PushBlob"}),
		pushDirectoryDurationSeconds: pushServerOperationsDurationSeconds.MustCurryWith(map[string]string{"name": name, "operation": "PushDirectory"}),
	}
}

func (s *metricsAssetPushServer) updateDurationSeconds(vec prometheus.ObserverVec, code codes.Code, timeStart time.Time, qualifiers []*remoteasset.Qualifier) {
	if len(qualifiers) == 0 {
		vec.WithLabelValues(code.String(), "N/A").Observe(s.clock.Now().Sub(timeStart).Seconds())
	} else {
		resourceType := "N/A"
		for _, qualifier := range qualifiers {
			if qualifier.Name == "resource_type" {
				resourceType = qualifier.Value
				break
			}
		}
		vec.WithLabelValues(code.String(), resourceType).Observe(s.clock.Now().Sub(timeStart).Seconds())
	}
}

func (s *metricsAssetPushServer) updateBlobSizeBytes(vec prometheus.ObserverVec, blobSize float64, qualifiers []*remoteasset.Qualifier) {
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

func (s *metricsAssetPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	if req.BlobDigest != nil {
		s.updateBlobSizeBytes(s.pushBlobBlobSizeBytes, float64(req.BlobDigest.SizeBytes), req.Qualifiers)
	}
	timeStart := s.clock.Now()
	resp, err := s.pushServer.PushBlob(ctx, req)
	s.updateDurationSeconds(s.pushBlobDurationSeconds, status.Code(err), timeStart, req.Qualifiers)
	return resp, err
}

func (s *metricsAssetPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	timeStart := s.clock.Now()
	resp, err := s.pushServer.PushDirectory(ctx, req)
	s.updateDurationSeconds(s.pushDirectoryDurationSeconds, status.Code(err), timeStart, req.Qualifiers)
	return resp, err
}
