package storage

import (
	"crypto/sha256"
	"encoding/hex"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"google.golang.org/protobuf/proto"

	"github.com/buildbarn/bb-storage/pkg/digest"
)

// EmptyDigest is a REv2 Digest representing an object of size 0 hashed
// with SHA256
var EmptyDigest = &remoteexecution.Digest{
	Hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	SizeBytes: 0,
}

// ProtoSerialise serialises an arbitrary protobuf message into its wire format and
// a Remote Execution API Digest of the format.  This is very useful for interacting
// with the Remote Execution API
func ProtoSerialise(pb proto.Message) ([]byte, *remoteexecution.Digest, error) {
	wireFormat, err := proto.Marshal(pb)
	if err != nil {
		return nil, nil, err
	}

	hash := sha256.Sum256(wireFormat)

	return wireFormat,
		&remoteexecution.Digest{
			Hash:      hex.EncodeToString(hash[:]),
			SizeBytes: int64(len(wireFormat)),
		}, nil
}

// ProtoToDigest converts an arbitrary protobuf message into a Buildbarn-internal
// Digest of its content.
func ProtoToDigest(pb proto.Message, instance digest.InstanceName) (digest.Digest, error) {
	_, reapiDigest, err := ProtoSerialise(pb)
	if err != nil {
		return digest.Digest{}, err
	}
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(reapiDigest.GetHash()))
	if err != nil {
		return digest.Digest{}, err
	}
	bbDigest, err := digestFunction.NewDigestFromProto(reapiDigest)
	if err != nil {
		return digest.Digest{}, err
	}
	return bbDigest, nil
}
