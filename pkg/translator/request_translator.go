package translator

import (
	"crypto/sha256"
	"encoding/hex"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/proto"
)

// EmptyDigest is the empty digest, used for convenience
var EmptyDigest *remoteexecution.Digest = &remoteexecution.Digest{
	Hash:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	SizeBytes: 0,
}

// RequestTranslator converts Remote Asset API requests into Actions
type RequestTranslator struct {
}

// URIsToAction converts the URIs from a Push{Blob,Directory}Request
// into a REAPI Action
func (rt *RequestTranslator) URIsToAction(uris []string) (remoteexecution.Action, remoteexecution.Command, error) {
	command := remoteexecution.Command{
		Arguments:   []string{"curl", "-o", "out", uris[0]},
		OutputPaths: []string{"out"},
	}
	commandDigest, err := ProtoToDigest(&command)
	if err != nil {
		return remoteexecution.Action{}, remoteexecution.Command{}, err
	}

	action := remoteexecution.Action{
		CommandDigest:   commandDigest,
		InputRootDigest: EmptyDigest,
	}

	return action, command, nil
}

// PushBlobToActionResult converts a PushBlobRequest into a REAPI ActionResult to be pushed into the Action Cache
func (rt *RequestTranslator) PushBlobToActionResult(req *remoteasset.PushBlobRequest) remoteexecution.ActionResult {
	actionResult := remoteexecution.ActionResult{
		OutputFiles: []*remoteexecution.OutputFile{
			{
				Path:   "out",
				Digest: req.BlobDigest,
			},
		},
		ExitCode: 0,
	}

	return actionResult
}

// PushDirectoryToActionResult converts a PushDirectoryRequest into a
// REAPI ActionResult to be pushed into the Action Cache
func (rt *RequestTranslator) PushDirectoryToActionResult(req *remoteasset.PushDirectoryRequest, treeDigest *remoteexecution.Digest) remoteexecution.ActionResult {
	actionResult := remoteexecution.ActionResult{
		OutputDirectories: []*remoteexecution.OutputDirectory{
			{
				Path:       "out",
				TreeDigest: treeDigest,
			},
		},
		ExitCode: 0,
	}

	return actionResult
}

// FetchBlobToAction converst a FetchBlobRequest into a REAPI Action
func (rt *RequestTranslator) FetchBlobToAction(req *remoteasset.FetchBlobRequest) (remoteexecution.Action, remoteexecution.Command, error) {
	command := remoteexecution.Command{
		Arguments:   []string{"curl", "-o", "out", req.Uris[0]},
		OutputPaths: []string{"out"},
	}
	commandDigest, err := ProtoToDigest(&command)
	if err != nil {
		return remoteexecution.Action{}, remoteexecution.Command{}, err
	}

	action := remoteexecution.Action{
		CommandDigest:   commandDigest,
		InputRootDigest: EmptyDigest,
	}

	return action, command, nil
}

// ProtoToDigest converts an arbitrary proto to a remote execution Digest
func ProtoToDigest(pb proto.Message) (*remoteexecution.Digest, error) {
	wireFormat, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(wireFormat)

	return &remoteexecution.Digest{
		Hash:      hex.EncodeToString(hash[:]),
		SizeBytes: int64(len(wireFormat)),
	}, nil
}
