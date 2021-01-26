package hostfrequencyscaler

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto/src -I . --gogo_out=plugin=grpc:. hostfrequencyscaler.proto

const Name = "type.googleapis.com/HostFrequencyScaler.Scaler"

/////////////////
// HostFrequencyScaler Object /
///////////////

var _ types.Extension = (*Scaler)(nil)

func (*Scaler) New() types.Message {
	return &Scaler{}
}

func (*Scaler) Name() string {
	return Name
}

func init() {
	core.Registry.RegisterExtension(&Scaler{})
}
