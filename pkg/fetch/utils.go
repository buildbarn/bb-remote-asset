package fetch

import (
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
)

// getDigestFunction gets the digest function specified by a request or uses SHA 256 by default
func getDigestFunction(digestFunction remoteexecution.DigestFunction_Value, instanceName string) (digest.Function, error) {
	instance, err := digest.NewInstanceName(instanceName)
	if err != nil {
		return digest.Function{}, util.StatusWrapf(err, "Invalid instance name %#v", instanceName)
	}

	// As per the API spec, default to SHA 256 if no digest function is set.
	if digestFunction == remoteexecution.DigestFunction_UNKNOWN {
		digestFunction = remoteexecution.DigestFunction_SHA256
	}

	// The value of the fallback hash length is unused, because we never have an
	// unknown value for the digest function enum.  Let's set it to 0 rather than
	// any actual value
	return instance.GetDigestFunction(digestFunction, 0)
}
