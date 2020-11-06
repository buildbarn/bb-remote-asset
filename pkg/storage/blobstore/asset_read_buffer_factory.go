package blobstore

import (
	"io"
	"math"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

type assetReadBufferFactory struct{}

func (f assetReadBufferFactory) NewBufferFromByteSlice(digest digest.Digest, data []byte, dataIntegrityCallback buffer.DataIntegrityCallback) buffer.Buffer {
	return buffer.NewProtoBufferFromByteSlice(&asset.Asset{}, data, buffer.BackendProvided(dataIntegrityCallback))
}

func (f assetReadBufferFactory) NewBufferFromReader(digest digest.Digest, r io.ReadCloser, dataIntegrityCallback buffer.DataIntegrityCallback) buffer.Buffer {
	return buffer.NewProtoBufferFromReader(&asset.Asset{}, r, buffer.BackendProvided(dataIntegrityCallback))
}

func (f assetReadBufferFactory) NewBufferFromReaderAt(digest digest.Digest, r buffer.ReadAtCloser, sizeBytes int64, dataIntegrityCallback buffer.DataIntegrityCallback) buffer.Buffer {
	return f.NewBufferFromReader(digest, newReaderFromReaderAt(r), dataIntegrityCallback)
}

func newReaderFromReaderAt(r buffer.ReadAtCloser) io.ReadCloser {
	return &struct {
		io.SectionReader
		io.Closer
	}{
		SectionReader: *io.NewSectionReader(r, 0, math.MaxInt64),
		Closer:        r,
	}
}

// AssetReadBufferFactory is capable of buffers for objects stored in
// the Asset Store.
var AssetReadBufferFactory blobstore.ReadBufferFactory = assetReadBufferFactory{}
