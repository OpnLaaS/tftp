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

