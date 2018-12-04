package ipmi

// RMCP constants
const (
	RMCPVersion1_0 uint8 = 0x06

	// Class bitmasks
	RMCPClassNormal uint8 = 0x00
	RMCPClassACK    uint8 = 0x80
	RMCPClassASF    uint8 = 0x06
	RMCPClassIPMI   uint8 = 0x07
	RMCPClassOEM    uint8 = 0x08

	RMCPSeqNoACK uint8 = 0xff
)

// ASF constants
const (
	ASFIANA              uint32 = 0x11be
	ASFTypePing          uint8  = 0x80
	ASFTypePong          uint8  = 0x40
	ASFTagUnidirectional uint8  = 0xff

	// bitmask
	ASFEntitiesIPMISupport uint8 = 0x80
	ASFEntitiesVersion1_0  uint8 = 0x01

	// bitmask
	ASFInteractionsRMCPSec  uint8 = 0x80
	ASFInteractionsDMTFDASH uint8 = 0x20
)

// IPMI NetFn codes
const (
	IPMIFnChassisReq   uint8 = 0x00
	IPMIFnChassisRes   uint8 = 0x01
	IPMIFnBridgeReq    uint8 = 0x02
	IPMIFnBridgeRes    uint8 = 0x03
	IPMIFnSensorReq    uint8 = 0x04
	IPMIFnSensorRes    uint8 = 0x05
	IPMIFnAppReq       uint8 = 0x06
	IPMIFnAppRes       uint8 = 0x07
	IPMIFnFirmwareReq  uint8 = 0x08
	IPMIFnFirmwareRes  uint8 = 0x09
	IPMIFnStorateReq   uint8 = 0x10
	IPMIFnStorageRes   uint8 = 0x11
	IPMIFnTransportReq uint8 = 0x12
	IPMIFnTransportRes uint8 = 0x13
	IPMIFnGroupReq     uint8 = 0x2c
	IPMIFnGroupRes     uint8 = 0x2d
	IPMIFnOEMReq       uint8 = 0x2e
	IPMIFnOEMRes       uint8 = 0x2f
	IPMIFnCtrlOEMReq   uint8 = 0x30
	IMPIFnCtrlOEMRes   uint8 = 0x31
)

// Completion codes
const (
	IPMICmpNorm    uint8 = 0x00
	IPMICmpBusy    uint8 = 0xc0
	IPMICmpInvalid uint8 = 0xc1
	//...
)

var IPMICmpString = map[uint8]string{
	IPMICmpNorm:    "Command completed normally.",
	IPMICmpBusy:    "Node Busy.",
	IPMICmpInvalid: "Invalid Command.",
	//...
}

// IPMI commands (very incomplete)
// limited to what we need to send basic power commands
const (
	// needed to manage sessions
	IPMICmdSendMessage    uint8 = 0x34
	IPMICmdGetChanAuthCap uint8 = 0x38
	IPMICmdGetSessionChal uint8 = 0x39
	IPMICmdActivateSess   uint8 = 0x3a
	IPMICmdSetSessionPriv uint8 = 0x3b
	IPMICmdCloseSess      uint8 = 0x3c

	IPMICmdChassisStatus       uint8 = 0x01
	IPMICmdChassisCtl          uint8 = 0x02
	IPMIChassisCtlDown         uint8 = 0x00
	IPMIChassisCtlUp           uint8 = 0x01
	IPMIChassisCtlCycle        uint8 = 0x02
	IPMIChassisCtlHardReset    uint8 = 0x03
	IPMIChassisCtlPulseDiag    uint8 = 0x04
	IPMIChassisCtlSoftShutdown uint8 = 0x05

	IMPIRsAddrBMCResponder uint8 = 0x20
)

// IPMI command data constants
const (
	IPMIGetChanAuthCapGetChannel uint8 = 0x0e
	IPMIPrivCallback             uint8 = 0x01
	IMPIPrivUser                 uint8 = 0x02
	IMPIPrivOperator             uint8 = 0x03
	IPMIPrivAdmin                uint8 = 0x04
	IPMIPrivOEM                  uint8 = 0x05

	// bitfield
	IPMIAuthTypeBFIPMI2  uint8 = 0x80
	IPMIAuthTypeBFOEM    uint8 = 0x20
	IPMIAuthTypeBFPasswd uint8 = 0x10
	IPMIAuthTypeBFMD5    uint8 = 0x04
	IPMIAuthTypeBFMD2    uint8 = 0x02
	IPMIAuthTypeBFNONE   uint8 = 0x01

	IPMIAuthTypeOEM    uint8 = 0x05
	IPMIAuthTypePasswd uint8 = 0x04
	IPMIAuthTypeMD5    uint8 = 0x02
	IPMIAuthTypeMD2    uint8 = 0x01
	IPMIAuthTypeNONE   uint8 = 0x00
)
