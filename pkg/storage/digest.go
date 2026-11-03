package storage

import (
	"google.golang.org/protobuf/proto"

	"github.com/buildbarn/bb-storage/pkg/digest"
)

// SerialiseProto serialises an arbitrary protobuf into a byte slice and
// computes its Digest using the passed digestFunction. This is handy
// for interfacing with REAPI via other Buildbarn APIs.
func SerialiseProto(pb proto.Message, digestFunction digest.Function) ([]byte, digest.Digest, error) {
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, digest.Digest{}, err
	}
	digestGenerator := digestFunction.NewGenerator(int64(len(data)))
	if _, err := digestGenerator.Write(data); err != nil {
		return nil, digest.Digest{}, err
	}
	return data, digestGenerator.Sum(), nil
}

// EmptyDigest produces the empty digest for the given DigestFunction
func EmptyDigest(digestFunction digest.Function) digest.Digest {
	generator := digestFunction.NewGenerator(int64(0))
	return generator.Sum()
}
