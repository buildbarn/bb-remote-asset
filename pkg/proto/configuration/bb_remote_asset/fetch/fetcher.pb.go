// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        v5.29.1
// source: pkg/proto/configuration/bb_remote_asset/fetch/fetcher.proto

package fetch

import (
	grpc "github.com/buildbarn/bb-storage/pkg/proto/configuration/grpc"
	http "github.com/buildbarn/bb-storage/pkg/proto/configuration/http"
	status "google.golang.org/genproto/googleapis/rpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type FetcherConfiguration struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Backend:
	//
	//	*FetcherConfiguration_Http
	//	*FetcherConfiguration_Error
	//	*FetcherConfiguration_RemoteExecution
	Backend isFetcherConfiguration_Backend `protobuf_oneof:"backend"`
}

func (x *FetcherConfiguration) Reset() {
	*x = FetcherConfiguration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetcherConfiguration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetcherConfiguration) ProtoMessage() {}

func (x *FetcherConfiguration) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetcherConfiguration.ProtoReflect.Descriptor instead.
func (*FetcherConfiguration) Descriptor() ([]byte, []int) {
	return file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescGZIP(), []int{0}
}

func (m *FetcherConfiguration) GetBackend() isFetcherConfiguration_Backend {
	if m != nil {
		return m.Backend
	}
	return nil
}

func (x *FetcherConfiguration) GetHttp() *FetcherConfiguration_HttpFetcherConfiguration {
	if x, ok := x.GetBackend().(*FetcherConfiguration_Http); ok {
		return x.Http
	}
	return nil
}

func (x *FetcherConfiguration) GetError() *status.Status {
	if x, ok := x.GetBackend().(*FetcherConfiguration_Error); ok {
		return x.Error
	}
	return nil
}

func (x *FetcherConfiguration) GetRemoteExecution() *FetcherConfiguration_RemoteExecutionFetcherConfiguration {
	if x, ok := x.GetBackend().(*FetcherConfiguration_RemoteExecution); ok {
		return x.RemoteExecution
	}
	return nil
}

type isFetcherConfiguration_Backend interface {
	isFetcherConfiguration_Backend()
}

type FetcherConfiguration_Http struct {
	Http *FetcherConfiguration_HttpFetcherConfiguration `protobuf:"bytes,2,opt,name=http,proto3,oneof"`
}

type FetcherConfiguration_Error struct {
	Error *status.Status `protobuf:"bytes,3,opt,name=error,proto3,oneof"`
}

type FetcherConfiguration_RemoteExecution struct {
	RemoteExecution *FetcherConfiguration_RemoteExecutionFetcherConfiguration `protobuf:"bytes,4,opt,name=remote_execution,json=remoteExecution,proto3,oneof"`
}

func (*FetcherConfiguration_Http) isFetcherConfiguration_Backend() {}

func (*FetcherConfiguration_Error) isFetcherConfiguration_Backend() {}

func (*FetcherConfiguration_RemoteExecution) isFetcherConfiguration_Backend() {}

type FetcherConfiguration_HttpFetcherConfiguration struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Client *http.ClientConfiguration `protobuf:"bytes,3,opt,name=client,proto3" json:"client,omitempty"`
}

func (x *FetcherConfiguration_HttpFetcherConfiguration) Reset() {
	*x = FetcherConfiguration_HttpFetcherConfiguration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetcherConfiguration_HttpFetcherConfiguration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetcherConfiguration_HttpFetcherConfiguration) ProtoMessage() {}

func (x *FetcherConfiguration_HttpFetcherConfiguration) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetcherConfiguration_HttpFetcherConfiguration.ProtoReflect.Descriptor instead.
func (*FetcherConfiguration_HttpFetcherConfiguration) Descriptor() ([]byte, []int) {
	return file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescGZIP(), []int{0, 0}
}

func (x *FetcherConfiguration_HttpFetcherConfiguration) GetClient() *http.ClientConfiguration {
	if x != nil {
		return x.Client
	}
	return nil
}

type FetcherConfiguration_RemoteExecutionFetcherConfiguration struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExecutionClient *grpc.ClientConfiguration `protobuf:"bytes,2,opt,name=execution_client,json=executionClient,proto3" json:"execution_client,omitempty"`
}

func (x *FetcherConfiguration_RemoteExecutionFetcherConfiguration) Reset() {
	*x = FetcherConfiguration_RemoteExecutionFetcherConfiguration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetcherConfiguration_RemoteExecutionFetcherConfiguration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetcherConfiguration_RemoteExecutionFetcherConfiguration) ProtoMessage() {}

func (x *FetcherConfiguration_RemoteExecutionFetcherConfiguration) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetcherConfiguration_RemoteExecutionFetcherConfiguration.ProtoReflect.Descriptor instead.
func (*FetcherConfiguration_RemoteExecutionFetcherConfiguration) Descriptor() ([]byte, []int) {
	return file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescGZIP(), []int{0, 1}
}

func (x *FetcherConfiguration_RemoteExecutionFetcherConfiguration) GetExecutionClient() *grpc.ClientConfiguration {
	if x != nil {
		return x.ExecutionClient
	}
	return nil
}

var File_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto protoreflect.FileDescriptor

var file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDesc = []byte{
	0x0a, 0x3b, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x62, 0x62, 0x5f, 0x72, 0x65, 0x6d,
	0x6f, 0x74, 0x65, 0x5f, 0x61, 0x73, 0x73, 0x65, 0x74, 0x2f, 0x66, 0x65, 0x74, 0x63, 0x68, 0x2f,
	0x66, 0x65, 0x74, 0x63, 0x68, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x2d, 0x62,
	0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x62, 0x62, 0x5f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65,
	0x5f, 0x61, 0x73, 0x73, 0x65, 0x74, 0x2e, 0x66, 0x65, 0x74, 0x63, 0x68, 0x1a, 0x17, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x72, 0x70, 0x63, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x27, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x67,
	0x72, 0x70, 0x63, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x27,
	0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x68, 0x74, 0x74, 0x70, 0x2f, 0x68, 0x74, 0x74,
	0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd7, 0x04, 0x0a, 0x14, 0x46, 0x65, 0x74, 0x63,
	0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x72, 0x0a, 0x04, 0x68, 0x74, 0x74, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x5c,
	0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x62, 0x62, 0x5f, 0x72, 0x65, 0x6d, 0x6f,
	0x74, 0x65, 0x5f, 0x61, 0x73, 0x73, 0x65, 0x74, 0x2e, 0x66, 0x65, 0x74, 0x63, 0x68, 0x2e, 0x46,
	0x65, 0x74, 0x63, 0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x48, 0x74, 0x74, 0x70, 0x46, 0x65, 0x74, 0x63, 0x68, 0x65, 0x72, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x48, 0x00, 0x52, 0x04,
	0x68, 0x74, 0x74, 0x70, 0x12, 0x2a, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x72, 0x70, 0x63,
	0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x48, 0x00, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72,
	0x12, 0x94, 0x01, 0x0a, 0x10, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x5f, 0x65, 0x78, 0x65, 0x63,
	0x75, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x67, 0x2e, 0x62, 0x75,
	0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x62, 0x62, 0x5f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x5f,
	0x61, 0x73, 0x73, 0x65, 0x74, 0x2e, 0x66, 0x65, 0x74, 0x63, 0x68, 0x2e, 0x46, 0x65, 0x74, 0x63,
	0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x52, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e,
	0x46, 0x65, 0x74, 0x63, 0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x48, 0x00, 0x52, 0x0f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x45, 0x78,
	0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x71, 0x0a, 0x18, 0x48, 0x74, 0x74, 0x70, 0x46,
	0x65, 0x74, 0x63, 0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x49, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x31, 0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x68, 0x74,
	0x74, 0x70, 0x2e, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4a, 0x04,
	0x08, 0x01, 0x10, 0x02, 0x4a, 0x04, 0x08, 0x02, 0x10, 0x03, 0x1a, 0x83, 0x01, 0x0a, 0x23, 0x52,
	0x65, 0x6d, 0x6f, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x46, 0x65,
	0x74, 0x63, 0x68, 0x65, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x12, 0x5c, 0x0a, 0x10, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x5f,
	0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x31, 0x2e, 0x62,
	0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x2e, 0x43, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x0f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x42, 0x09, 0x0a, 0x07, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x4a, 0x04, 0x08, 0x01, 0x10,
	0x02, 0x42, 0x54, 0x5a, 0x52, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2f, 0x62, 0x62, 0x2d, 0x72, 0x65, 0x6d,
	0x6f, 0x74, 0x65, 0x2d, 0x61, 0x73, 0x73, 0x65, 0x74, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x62, 0x62, 0x5f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x5f, 0x61, 0x73, 0x73, 0x65,
	0x74, 0x2f, 0x66, 0x65, 0x74, 0x63, 0x68, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescOnce sync.Once
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescData = file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDesc
)

func file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescGZIP() []byte {
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescOnce.Do(func() {
		file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescData)
	})
	return file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDescData
}

var file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_goTypes = []interface{}{
	(*FetcherConfiguration)(nil),                                     // 0: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration
	(*FetcherConfiguration_HttpFetcherConfiguration)(nil),            // 1: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.HttpFetcherConfiguration
	(*FetcherConfiguration_RemoteExecutionFetcherConfiguration)(nil), // 2: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.RemoteExecutionFetcherConfiguration
	(*status.Status)(nil),                                            // 3: google.rpc.Status
	(*http.ClientConfiguration)(nil),                                 // 4: buildbarn.configuration.http.ClientConfiguration
	(*grpc.ClientConfiguration)(nil),                                 // 5: buildbarn.configuration.grpc.ClientConfiguration
}
var file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_depIdxs = []int32{
	1, // 0: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.http:type_name -> buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.HttpFetcherConfiguration
	3, // 1: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.error:type_name -> google.rpc.Status
	2, // 2: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.remote_execution:type_name -> buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.RemoteExecutionFetcherConfiguration
	4, // 3: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.HttpFetcherConfiguration.client:type_name -> buildbarn.configuration.http.ClientConfiguration
	5, // 4: buildbarn.configuration.bb_remote_asset.fetch.FetcherConfiguration.RemoteExecutionFetcherConfiguration.execution_client:type_name -> buildbarn.configuration.grpc.ClientConfiguration
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_init() }
func file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_init() {
	if File_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetcherConfiguration); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetcherConfiguration_HttpFetcherConfiguration); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetcherConfiguration_RemoteExecutionFetcherConfiguration); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*FetcherConfiguration_Http)(nil),
		(*FetcherConfiguration_Error)(nil),
		(*FetcherConfiguration_RemoteExecution)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_goTypes,
		DependencyIndexes: file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_depIdxs,
		MessageInfos:      file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_msgTypes,
	}.Build()
	File_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto = out.File
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_rawDesc = nil
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_goTypes = nil
	file_pkg_proto_configuration_bb_remote_asset_fetch_fetcher_proto_depIdxs = nil
}
