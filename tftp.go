package tftp

import (
	"context"
	"fmt"
	"net"
	"time"
)

func sendErrorTFTP(conn *net.UDPConn, addr *net.UDPAddr, errCode int, errMsg string) (err error) {
	var buffer []byte = make([]byte, 5+len(errMsg))

	buffer[0] = 0
	buffer[1] = OPCODE_ERROR
	buffer[2] = 0
	buffer[3] = byte(errCode)
	copy(buffer[4:], errMsg)
	buffer[4+len(errMsg)] = 0

	_, err = conn.WriteToUDP(buffer, addr)
	return
}

func ParseRRQRequestTFTP(buffer []byte) (file string, mode string, err error) {
	var (
		start int      = 2
		parts []string = make([]string, 0)
	)

	for i := 2; i < len(buffer); i++ {
		if buffer[i] == 0 {
			parts = append(parts, string(buffer[start:i]))
			start = i + 1
		}
	}

	if len(parts) < 2 {
		err = fmt.Errorf("invalid request")
		return
	}

	file = parts[0]
	mode = parts[1]
	return
}

func SendBufferTFTP(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, content []byte) error {
	blockNum := uint16(1)
	offset := 0

	// A zero-length file still requires a single data/ack exchange.
	if len(content) == 0 {
		content = []byte{}
	}

	for {
		chunkEnd := offset + BLOCK_SIZE
		if chunkEnd > len(content) {
			chunkEnd = len(content)
		}

		chunk := content[offset:chunkEnd]
		packet := make([]byte, 4+len(chunk))
		packet[0] = 0
		packet[1] = OPCODE_DATA
		packet[2] = byte(blockNum >> 8)
		packet[3] = byte(blockNum)
		copy(packet[4:], chunk)

		if _, err := conn.WriteToUDP(packet, addr); err != nil {
			return err
		}

		ack := make([]byte, 4)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if _, _, err := conn.ReadFromUDP(ack); err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						// resend last packet on timeout
						if _, err := conn.WriteToUDP(packet, addr); err != nil {
							return err
						}
						continue
					}
				}
				return err
			}

			if ack[1] == OPCODE_ACK && ack[2] == byte(blockNum>>8) && ack[3] == byte(blockNum) {
				break
			}
		}

		offset = chunkEnd
		if offset >= len(content) && len(chunk) < BLOCK_SIZE {
			return nil
		}

		blockNum++
	}
}
