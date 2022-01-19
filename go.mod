module github.com/buildbarn/bb-remote-asset

go 1.16

replace github.com/gordonklaus/ineffassign => github.com/gordonklaus/ineffassign v0.0.0-20201223204552-cba2d2a1d5d9

require (
	github.com/bazelbuild/remote-apis v0.0.0-20211004185116-636121a32fa7
	github.com/buildbarn/bb-storage v0.0.0-20211205205823-634fb8ef62e0
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/prometheus/client_golang v1.11.0
	github.com/stretchr/testify v1.7.0
	google.golang.org/genproto v0.0.0-20211104193956-4c6863e31247
	google.golang.org/grpc v1.42.0
)
