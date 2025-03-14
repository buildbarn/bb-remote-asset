package storage

import (
	"google.golang.org/protobuf/proto"

	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// ProtoSerialise serialises an arbitrary protobuf into a buffer containing its
// wire format and a Digest computed using the passed digestFunction.  This is
// handy for interfacing with REAPI via other Buildbarn APIs.
func ProtoSerialise(pb proto.Message, digestFunction digest.Function) (buffer.Buffer, digest.Digest, error) {
	buf := buffer.NewProtoBufferFromProto(pb, buffer.UserProvided)

	sizeBytes, err := buf.GetSizeBytes()
	if err != nil {
		return nil, digest.Digest{}, err
	}

	raw, err := buf.ToByteSlice(int(sizeBytes))
	if err != nil {
		return nil, digest.Digest{}, err
	}

	digestGenerator := digestFunction.NewGenerator(sizeBytes)
	// TODO: Ensure we write the full buffer
	_, err = digestGenerator.Write(raw)
	if err != nil {
		return nil, digest.Digest{}, err
	}

	return buf, digestGenerator.Sum(), nil
}

// EmptyDigest produces the empty digest for the given DigestFunction
func EmptyDigest(digestFunction digest.Function) digest.Digest {
	generator := digestFunction.NewGenerator(int64(0))
	return generator.Sum()
}
