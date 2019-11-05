package hostfrequencyscaler

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/extensions/HostFrequencyScaler/proto"
	"github.com/hpc/kraken/lib"
)

//go:generate protoc -I ../../core/proto/include -I proto --go_out=plugins=grpc:proto proto/RFAggregatorServer.proto

/////////////////
// RFAggregatorServer Object /
///////////////

var _ lib.Extension = HostFrequencyScaler{}

type HostFrequencyScaler struct{}

func (HostFrequencyScaler) New() proto.Message {
	return &pb.HostFrequencyScaler{}
}

func (r HostFrequencyScaler) Name() string {
	a, _ := ptypes.MarshalAny(r.New())
	return a.GetTypeUrl()
}

func init() {
	core.Registry.RegisterExtension(HostFrequencyScaler{})
}
