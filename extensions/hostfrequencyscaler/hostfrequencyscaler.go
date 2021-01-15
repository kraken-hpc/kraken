package hostfrequencyscaler

import (
	"github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/json"
	"github.com/hpc/kraken/lib/types"
)

//go:generate protoc -I ../../core/proto -I . --go_out=plugins=grpc:. hostfrequencyscaler.proto

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

// MarshalJSON creats a JSON version of Node
func (s *Scaler) MarshalJSON() ([]byte, error) {
	return json.MarshalJSON(s)
}

// UnmarshalJSON populates a node from JSON
func (s *Scaler) UnmarshalJSON(j []byte) error {
	return json.UnmarshalJSON(j, s)
}

func init() {
	core.Registry.RegisterExtension(&Scaler{})
}
