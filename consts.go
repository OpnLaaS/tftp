package tftp

const (
	BLOCK_SIZE   = 512
	OPCODE_RRQ   = 1
	OPCODE_DATA  = 3
	OPCODE_ACK   = 4
	OPCODE_ERROR = 5
)

const (
	GetTypeTFTP GetType = iota
	GetTypeHTTP
)

const (
	dhcpOpRequest = 1
	dhcpOpReply   = 2

	dhcpOptionMessageType   = 53
	dhcpOptionServerID      = 54
	dhcpOptionRequestedIP   = 50
	dhcpOptionLeaseTime     = 51
	dhcpOptionSubnetMask    = 1
	dhcpOptionRouter        = 3
	dhcpOptionDNSServer     = 6
	dhcpOptionDomainName    = 15
	dhcpOptionBootFileName  = 67
	dhcpOptionTFTPServer    = 66
	dhcpOptionEnd           = 255

	dhcpMessageDiscover = 1
	dhcpMessageOffer    = 2
	dhcpMessageRequest  = 3
	dhcpMessageDecline  = 4
	dhcpMessageAck      = 5
	dhcpMessageNak      = 6
	dhcpMessageRelease  = 7
	dhcpMessageInform   = 8
)
