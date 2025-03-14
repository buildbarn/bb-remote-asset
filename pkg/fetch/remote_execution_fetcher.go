package fetch

import (
	"context"
	"log"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type remoteExecutionFetcher struct {
	contentAddressableStorage blobstore.BlobAccess
	executionClient           remoteexecution.ExecutionClient
	maximumMessageSizeBytes   int
}

// NewRemoteExecutionFetcher creates a new Fetcher that is capable of
// itself fetching resources from other places (as defined in the
// qualifier_translator).
func NewRemoteExecutionFetcher(contentAddressableStorage blobstore.BlobAccess, client grpc.ClientConnInterface, maximumMessageSizeBytes int) Fetcher {
	return &remoteExecutionFetcher{
		contentAddressableStorage: contentAddressableStorage,
		executionClient:           remoteexecution.NewExecutionClient(client),
		maximumMessageSizeBytes:   maximumMessageSizeBytes,
	}
}

// fetchCommon converts a FetchBlobRequest into an Action, runs the Action using
// the Execution instance configured, then fetches the ActionResult obtained.
// It returns the ActionResult, the URI that was fetched, the path to the output,
// and an error value.
func (rf *remoteExecutionFetcher) fetchCommon(ctx context.Context, req *remoteasset.FetchBlobRequest, digestFunction digest.Function) (*remoteexecution.ActionResult, string, string, error) {
	// Get the Command Generator for the set of qualifiers in the request
	commandGenerator, err := qualifier.QualifiersToCommand(req.Qualifiers)
	if err != nil {
		return nil, "", "", err
	}

	// Attempt to download each URI in turn
	for _, uri := range req.Uris {
		// Convert URI into an Action (and Command) based on the qualifiers set
		command := commandGenerator(uri)
		commandPb, commandDigest, err := storage.ProtoSerialise(command, digestFunction)
		if err != nil {
			return nil, "", "", err
		}
		action := &remoteexecution.Action{
			CommandDigest:   commandDigest.GetProto(),
			InputRootDigest: storage.EmptyDigest(digestFunction).GetProto(),
		}
		actionPb, actionDigest, err := storage.ProtoSerialise(action, digestFunction)
		if err != nil {
			return nil, "", "", err
		}

		// Upload Action and Command to the CAS
		err = rf.contentAddressableStorage.Put(ctx, actionDigest, actionPb)
		if err != nil {
			return nil, "", "", err
		}
		err = rf.contentAddressableStorage.Put(ctx, commandDigest, commandPb)
		if err != nil {
			return nil, "", "", err
		}

		// Execute the fetch using the Execution service
		stream, err := rf.executionClient.Execute(ctx, &remoteexecution.ExecuteRequest{
			InstanceName: req.InstanceName,
			ActionDigest: actionDigest.GetProto(),
		})
		if err != nil {
			return nil, "", "", err
		}
		response := &remoteexecution.ExecuteResponse{}
		for {
			operation, err := stream.Recv()
			if err != nil {
				return nil, "", "", err
			}
			if operation.GetDone() {
				err = anypb.UnmarshalTo(operation.GetResponse(), response, proto.UnmarshalOptions{})
				if err != nil {
					return nil, "", "", err
				}
				break
			}
		}

		// Check the result
		actionResult := response.GetResult()
		if exitCode := actionResult.GetExitCode(); exitCode != 0 {
			log.Printf("Remote execution fetch was unsuccessful for URI: %v", uri)
			continue
		}
		return actionResult, uri, command.OutputPaths[0], nil
	}

	// No URI was successful
	return nil, "", "", status.Errorf(codes.NotFound, "Unable to download blob from any of the provided URIs")
}

func (rf *remoteExecutionFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	// Get the Digest Function for this request
	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName)
	if err != nil {
		return nil, err
	}

	// Execute all possible fetches using the Execution service
	actionResult, uri, outputPath, err := rf.fetchCommon(ctx, req, digestFunction)
	if err != nil {
		return nil, err
	}

	// Find the digest corresponding to the output path we want
	digest := storage.EmptyDigest(digestFunction).GetProto()
	for _, file := range actionResult.GetOutputFiles() {
		if file.Path == outputPath {
			digest = file.GetDigest()
		}
	}

	// If we got no match, check the directories obtained.  If the output path expected
	// is a directory, give a nicer error to the user.
	if digest.GetSizeBytes() == 0 {
		for _, directory := range actionResult.GetOutputDirectories() {
			if directory.Path == outputPath {
				fetchErr := status.New(codes.Aborted, "Expected blob but downloaded directory")
				return &remoteasset.FetchBlobResponse{
					Status:     fetchErr.Proto(),
					Uri:        uri,
					Qualifiers: req.Qualifiers,
				}, fetchErr.Err()
			}
		}
		return nil, status.Errorf(codes.NotFound, "Unable to fetch blob from any of the URIs specified")
	}

	return &remoteasset.FetchBlobResponse{
		Status:     status.New(codes.OK, "Blob fetched successfully!").Proto(),
		Uri:        uri,
		Qualifiers: req.Qualifiers,
		BlobDigest: digest,
	}, nil
}

func (rf *remoteExecutionFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	// Get the Digest Function for this request
	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName)
	if err != nil {
		return nil, err
	}

	// Execute all possible fetches using the Execution service
	// We convert to a Blob request first to use the shared fetchCommon
	blobReq := &remoteasset.FetchBlobRequest{
		InstanceName: req.InstanceName,
		Uris:         req.Uris,
		Qualifiers:   req.Qualifiers,
	}
	actionResult, uri, outputPath, err := rf.fetchCommon(ctx, blobReq, digestFunction)
	if err != nil {
		return nil, err
	}

	// Find the digest corresponding to the output path we want
	digest := storage.EmptyDigest(digestFunction).GetProto()
	for _, directory := range actionResult.GetOutputDirectories() {
		if directory.Path == outputPath {
			digest = directory.GetTreeDigest()
		}
	}

	// If we didn't find a directory for the output path, check the files.  If we hit
	// the path as a file, present a nicer error for the user.
	if digest.GetSizeBytes() == 0 {
		for _, file := range actionResult.GetOutputFiles() {
			if file.Path == outputPath {
				fetchErr := status.New(codes.Aborted, "Expected directory but downloaded file")
				return &remoteasset.FetchDirectoryResponse{
					Status:     fetchErr.Proto(),
					Uri:        uri,
					Qualifiers: req.Qualifiers,
				}, fetchErr.Err()
			}
		}
		return nil, status.Errorf(codes.NotFound, "Unable to fetch directory from any of the URIs specified")
	}

	// ActionResults point to Tree digests, but the Remote Asset API expects
	// the Digest of the root Directory proto.  Download the Tree so we can
	// find the corresponding Directory.
	treeDigest, err := digestFunction.NewDigestFromProto(digest)
	if err != nil {
		return nil, err
	}
	tree, err := rf.contentAddressableStorage.Get(ctx, treeDigest).ToProto(&remoteexecution.Tree{}, rf.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	root := tree.(*remoteexecution.Tree).Root

	// Ensure the tree's Directory protos are in the CAS
	rootPb, rootDigest, err := storage.ProtoSerialise(root, digestFunction)
	if err != nil {
		return nil, err
	}
	err = rf.contentAddressableStorage.Put(ctx, rootDigest, rootPb)
	if err != nil {
		return nil, err
	}
	for _, child := range tree.(*remoteexecution.Tree).Children {
		childPb, childDigest, err := storage.ProtoSerialise(child, digestFunction)
		if err != nil {
			return nil, err
		}
		err = rf.contentAddressableStorage.Put(ctx, childDigest, childPb)
		if err != nil {
			return nil, err
		}
	}

	return &remoteasset.FetchDirectoryResponse{
		Status:              status.New(codes.OK, "Directory fetched successfully!").Proto(),
		Uri:                 uri,
		Qualifiers:          req.Qualifiers,
		RootDirectoryDigest: rootDigest.GetProto(),
	}, nil
}

func (rf *remoteExecutionFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Difference(qualifiers, qualifier.NewSet([]string{"resource_type", "vcs.branch", "vcs.commit", "auth.basic.username", "auth.basic.password", "checksum.sri"}))
}
