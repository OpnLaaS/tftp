package tftp

import (
	"fmt"
	"net"
	"os"
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

func parseRQQRequestTFTP(buffer []byte) (file string, mode string, err error) {
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

func sendFileTFTP(conn *net.UDPConn, addr *net.UDPAddr, filename string) (err error) {
	var file *os.File

	if file, err = os.Open(filename); err != nil {
		sendErrorTFTP(conn, addr, 1, "File not found")
		return
	}

	defer file.Close()

	var (
		bytesRead int    = 0
		blockNum  uint16 = 1
		buffer    []byte = make([]byte, BLOCK_SIZE)
	)

	for {
		if bytesRead, err = file.Read(buffer); err != nil {
			return
		}

		var dataPacket []byte = make([]byte, 4+bytesRead)
		dataPacket[0] = 0
		dataPacket[1] = OPCODE_DATA
		dataPacket[2] = byte(blockNum >> 8)
		dataPacket[3] = byte(blockNum)
		copy(dataPacket[4:], buffer[:bytesRead])

		if _, err = conn.WriteToUDP(dataPacket, addr); err != nil {
			return
		}

		var ack []byte = make([]byte, 4)
		if _, _, err = conn.ReadFromUDP(ack); err != nil {
			return
		}

		if ack[1] != OPCODE_ACK || ack[2] != byte(blockNum>>8) || ack[3] != byte(blockNum) {
			err = fmt.Errorf("invalid ACK received: %v", ack)
			return
		}

		blockNum++
		if bytesRead < BLOCK_SIZE {
			return
		}
	}
}

func serveTFTP(options Options) {
	var (
		address *net.UDPAddr
		conn    *net.UDPConn
		err     error
	)

	if address, err = net.ResolveUDPAddr("udp4", options.ListenAddrTFTP); err != nil {
		panic(err)
	}

	if conn, err = net.ListenUDP("udp4", address); err != nil {
		panic(err)
	}

	go func() {
		defer conn.Close()

		var (
			buffer            []byte = make([]byte, 1024)
			bytesRead, opCode int    = 0, OPCODE_ERROR
			filename          string
			err               error
			clientAddr        *net.UDPAddr
		)

		for {
			if bytesRead, clientAddr, err = conn.ReadFromUDP(buffer); err != nil {
				continue
			}

			if bytesRead < 4 {
				continue
			}

			opCode = int(buffer[1])

			if opCode == OPCODE_RRQ {
				if filename, _, err = parseRQQRequestTFTP(buffer[:bytesRead]); err != nil {
					sendErrorTFTP(conn, clientAddr, 0, "Invalid request")
					continue
				}

				if err = sendFileTFTP(conn, clientAddr, filename); err != nil {
					continue
				}
			}
		}
	}()
}
