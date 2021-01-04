package storage

import remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

// EmptyDigest is a REv2 Digest representing an object of size 0 hashed
// with SHA256
var EmptyDigest = &remoteexecution.Digest{
	Hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	SizeBytes: 0,
}
