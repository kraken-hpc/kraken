package ipmi

type RMCPHeader struct {
	Version        uint8  `pack:""`
	reserved       uint8  `pack:"zeros"`
	SequenceNumber uint8  `pack:""`
	Class          uint8  `pack:""`
	Data           []byte `pack:"fill=0"`
}

type ASFMessageHeader struct {
	IANA     uint32 `pack:""`
	Type     uint8  `pack:""`
	Tag      uint8  `pack:""`
	reserved uint8  `pack:"zeros=true"`
	DataLen  uint8  `pack:"len=Data"`
	Data     []byte `pack:"fill=0"`
}

type ASFMessagePong struct {
	IANA         uint32  `pack:""`
	OEM          uint32  `pack:""`
	Entities     uint8   `pack:""`
	Interactions uint8   `pack:""`
	reserved     [6]byte `pack:"zeros"`
}

type IPMISessionHeader struct {
	AuthType              uint8  `pack:""`
	SessionSequenceNumber uint32 `pack:""`
	SessionID             uint32 `pack:""`
	MsgAuthCode           []byte `pack:"authcodelen=AuthType"` // a special one-off
	PayloadLength         uint8  `pack:"len=Payload"`
	Payload               []byte `pack:"fill=0"`
}

type IPMIMessageHeader struct {
	RsAddr   uint8  `pack:""`
	NetFnLun uint8  `pack:""`
	Checksum uint8  `pack:"cksum2=0"`
	Data     []byte `pack:"fill=0"`
}

type IPMIRequest struct {
	RqAddr   uint8  `pack:""`
	RqSeq    uint8  `pack:""`
	Cmd      uint8  `pack:""`
	Data     []byte `pack:"fill=-1"`
	Checksum uint8  `pack:"cksum2=0"`
}

type IPMIResponse struct {
	RqAddr   uint8  `pack:""`
	RqSeq    uint8  `pack:""`
	Cmd      uint8  `pack:""`
	CompCode uint8  `pack:""`
	Data     []byte `pack:"fill=-1"`
	Checksum uint8  `pack:"cksum2=0"`
}
