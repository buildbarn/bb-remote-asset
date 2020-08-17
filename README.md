# Buildbarn Remote Asset - Prototype

**N.B** This repository provides tools which are in early development and may be subject to regular changes to functionality and/or configuration definitions.

This repository provides a service for the [remote asset](https://github.com/bazelbuild/remote-apis/blob/master/build/bazel/remote/asset/v1/remote_asset.proto) protocol.
This protocol is used by tools such as [bazel](https://github.com/bazelbuild/bazel) /
[buildstream](https://gitlab.com/BuildStream/buildstream) to provide a mapping
between URIs and qualifiers to digests which can be used by the [remote execution](https://github.com/bazelbuild/remote-apis/blob/master/build/bazel/remote/execution/v2/remote_execution.proto) (REv2) protocol.

The remote asset daemon can be configured with [bb-storage](https://github.com/buildbarn/bb-storage) blobstore backends to
enable a scalable remote asset service which can be integrated with any REv2 compatible GRPC cache.

## Setting up the Remote Asset daemon

```
$ cat config/bb_remote_asset.jsonnet
{
  fetcher: {
    caching: {
      fetcher: {
        http: {
          allowUpdatesForInstances: ['foo'],
          contentAddressableStorage: {
            grpc: {
              address: "<cache_address>:<cache grpc port>"
            },
    }}}}},

  assetStore: {
    circular: {
      directory: '/storage-asset',
      offsetFileSizeBytes: 1024 * 1024,
      offsetCacheSize: 1000,
      dataFileSizeBytes: 100 * 1024 * 1024,
      dataAllocationChunkSizeBytes: 1048576,
      instances: ['foo'],
    },
  },
  httpListenAddress: ':7982',
  grpcServers: [{
    listenAddresses: [':8981'],
    authenticationPolicy: { allow: {} },
  }],
  allowUpdatesForInstances: ['foo'],
  maximumMessageSizeBytes: 16 * 1024 * 1024 * 1024,
}

$ docker run \
    -p 8981:8981 \
    -v $(pwd)/config:/config \
    -v $(pwd)/storage-asset:/storage-asset \
    bazel/cmd/bb_remote_asset:bb_remote_asset_container \
    /config/bb_remote_asset.jsonnet
```

In the example above, the daemon is configured to store asset references within a
disk backed circular storage backend. The fetcher is configured to support fetching via HTTP
when a reference is not found matching the request URI/Qualifier criteria, these fetched blobs are
placed into a REv2 compatible GRPC cache and the digest returned to the remote asset client.
HTTP Fetched blobs are configured to be cached references to newly fetched blobs
in the asset store for future fetches.

Bazel can be configured to use this service as a remote uploader as follows:

`$ bazel build --remote_cache=grpc://<cache_address>:<cache grpc port> --remote_instance_name=foo --experimental_remote_downloader="grpc://localhost:8981" //...`
