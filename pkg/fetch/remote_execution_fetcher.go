package fetch

import (
	"context"
	"log"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
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

func (rf *remoteExecutionFetcher) fetchCommon(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteexecution.ActionResult, string, string, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, "", "", err
	}
	commandGenerator, err := qualifier.QualifiersToCommand(req.Qualifiers)
	if err != nil {
		return nil, "", "", err
	}
	for _, uri := range req.Uris {
		command := commandGenerator(uri)
		commandDigest, err := storage.ProtoToDigest(command)
		if err != nil {
			return nil, "", "", err
		}
		commandDigestFunction, err := instanceName.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(commandDigest.GetHash()))
		if err != nil {
			return nil, "", "", err
		}

		action := &remoteexecution.Action{
			CommandDigest:   commandDigest,
			InputRootDigest: storage.EmptyDigest,
		}
		actionDigest, err := storage.ProtoToDigest(action)
		if err != nil {
			return nil, "", "", err
		}

		actionPb, err := proto.Marshal(action)
		if err != nil {
			return nil, "", "", err
		}

		commandPb, err := proto.Marshal(command)
		if err != nil {
			return nil, "", "", err
		}

		actionDigestFunction, err := instanceName.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(actionDigest.GetHash()))
		if err != nil {
			return nil, "", "", err
		}

		bbActionDigest, err := actionDigestFunction.NewDigestFromProto(actionDigest)
		if err != nil {
			return nil, "", "", err
		}
		err = rf.contentAddressableStorage.Put(ctx, bbActionDigest, buffer.NewCASBufferFromByteSlice(bbActionDigest, actionPb, buffer.UserProvided))
		if err != nil {
			return nil, "", "", err
		}

		bbCommandDigest, err := commandDigestFunction.NewDigestFromProto(commandDigest)
		if err != nil {
			return nil, "", "", err
		}
		err = rf.contentAddressableStorage.Put(ctx, bbCommandDigest, buffer.NewCASBufferFromByteSlice(bbCommandDigest, commandPb, buffer.UserProvided))
		if err != nil {
			return nil, "", "", err
		}

		stream, err := rf.executionClient.Execute(ctx, &remoteexecution.ExecuteRequest{
			InstanceName: req.InstanceName,
			ActionDigest: actionDigest,
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
				err = anypb.UnmarshalFrom(operation.GetResponse(), response)
				if err != nil {
					return nil, "", "", err
				}
				break
			}
		}

		actionResult := response.GetResult()
		if exitCode := actionResult.GetExitCode(); exitCode != 0 {
			log.Printf("Remote execution fetch was unsuccessful for URI: %v", uri)
			continue
		}
		return actionResult, uri, command.OutputPaths[0], nil
	}
	return nil, "", "", status.Errorf(codes.NotFound, "Unable to download blob from any of the provided URIs")
}

func (rf *remoteExecutionFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	actionResult, uri, outputPath, err := rf.fetchCommon(ctx, req)
	if err != nil {
		return nil, err
	}
	digest := storage.EmptyDigest
	for _, file := range actionResult.GetOutputFiles() {
		if file.Path == outputPath {
			digest = file.GetDigest()
		}
	}
	if digest == storage.EmptyDigest {
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
	blobReq := &remoteasset.FetchBlobRequest{
		InstanceName: req.InstanceName,
		Uris:         req.Uris,
		Qualifiers:   req.Qualifiers,
	}
	actionResult, uri, outputPath, err := rf.fetchCommon(ctx, blobReq)
	if err != nil {
		return nil, err
	}
	instance, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	digest := storage.EmptyDigest
	for _, directory := range actionResult.GetOutputDirectories() {
		if directory.Path == outputPath {
			digest = directory.GetTreeDigest()
		}
	}
	if digest == storage.EmptyDigest {
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
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(digest.GetHash()))
	if err != nil {
		return nil, err
	}
	treeDigest, err := digestFunction.NewDigestFromProto(digest)
	if err != nil {
		return nil, err
	}
	tree, err := rf.contentAddressableStorage.Get(ctx, treeDigest).ToProto(&remoteexecution.Tree{}, rf.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	root := tree.(*remoteexecution.Tree).Root
	rootDigest, err := storage.ProtoToDigest(root)
	if err != nil {
		return nil, err
	}
	bbRootDigest, err := digestFunction.NewDigestFromProto(rootDigest)
	if err != nil {
		return nil, err
	}
	err = rf.contentAddressableStorage.Put(ctx, bbRootDigest, buffer.NewProtoBufferFromProto(root, buffer.UserProvided))
	if err != nil {
		return nil, err
	}
	for _, child := range tree.(*remoteexecution.Tree).Children {
		childDigest, err := storage.ProtoToDigest(child)
		if err != nil {
			return nil, err
		}
		bbChildDigest, err := digestFunction.NewDigestFromProto(childDigest)
		if err != nil {
			return nil, err
		}
		err = rf.contentAddressableStorage.Put(ctx, bbChildDigest, buffer.NewProtoBufferFromProto(child, buffer.UserProvided))
		if err != nil {
			return nil, err
		}
	}
	return &remoteasset.FetchDirectoryResponse{
		Status:              status.New(codes.OK, "Directory fetched successfully!").Proto(),
		Uri:                 uri,
		Qualifiers:          req.Qualifiers,
		RootDirectoryDigest: rootDigest,
	}, nil
}

func (rf *remoteExecutionFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Difference(qualifiers, qualifier.NewSet([]string{"resource_type", "vcs.branch", "vcs.commit", "auth.basic.username", "auth.basic.password", "checksum.sri"}))
}
