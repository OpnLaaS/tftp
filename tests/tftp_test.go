package tftp_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"

	"github.com/opnlaas/tftp"
)

func TestParseRRQRequestTFTP(t *testing.T) {
	buffer := []byte{0, tftp.OPCODE_RRQ}
	buffer = append(buffer, []byte("pxelinux.0")...)
	buffer = append(buffer, 0)
	buffer = append(buffer, []byte("octet")...)
	buffer = append(buffer, 0)

	file, mode, err := tftp.ParseRRQRequestTFTP(buffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if file != "pxelinux.0" {
		t.Fatalf("expected file pxelinux.0, got %s", file)
	}

	if mode != "octet" {
		t.Fatalf("expected mode octet, got %s", mode)
	}
}

func TestSendBufferTFTP(t *testing.T) {
	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to open client conn: %v", err)
	}
	defer clientConn.Close()

	content := []byte("hello world")
	collected := make(chan []byte, 1)

	go func() {
		var all []byte
		buf := make([]byte, 2048)
		for {
			n, addr, err := clientConn.ReadFromUDP(buf)
			if err != nil {
				collected <- all
				return
			}

			if n < 4 {
				collected <- all
				return
			}

			blockNum := binary.BigEndian.Uint16(buf[2:4])
			ack := []byte{0, tftp.OPCODE_ACK, buf[2], buf[3]}
			if _, err := clientConn.WriteToUDP(ack, addr); err != nil {
				collected <- all
				return
			}

			all = append(all, buf[4:n]...)

			if n-4 < tftp.BLOCK_SIZE {
				// Last chunk received.
				collected <- all
				return
			}

			// Avoid runaway loop in case of unexpected block numbering.
			if blockNum > 5 {
				collected <- all
				return
			}
		}
	}()

	dataConn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatalf("failed to open data conn: %v", err)
	}
	defer dataConn.Close()

	err = tftp.SendBufferTFTP(context.Background(), dataConn, clientConn.LocalAddr().(*net.UDPAddr), content)
	if err != nil {
		t.Fatalf("sendBufferTFTP returned error: %v", err)
	}

	received := <-collected
	if !bytes.Equal(received, content) {
		t.Fatalf("content mismatch: got %q, want %q", received, content)
	}
}

func TestSendBufferTFTPZeroLength(t *testing.T) {
	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to open client conn: %v", err)
	}
	defer clientConn.Close()

	acks := make(chan int, 1)
	go func() {
		buf := make([]byte, 2048)
		n, addr, err := clientConn.ReadFromUDP(buf)
		if err != nil || n < 4 {
			acks <- 0
			return
		}

		ack := []byte{0, tftp.OPCODE_ACK, buf[2], buf[3]}
		_, _ = clientConn.WriteToUDP(ack, addr)
		acks <- n
	}()

	dataConn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatalf("failed to open data conn: %v", err)
	}
	defer dataConn.Close()

	err = tftp.SendBufferTFTP(context.Background(), dataConn, clientConn.LocalAddr().(*net.UDPAddr), []byte{})
	if err != nil {
		t.Fatalf("sendBufferTFTP returned error: %v", err)
	}

	if first := <-acks; first == 0 {
		t.Fatalf("expected at least one packet to be sent for empty content")
	}
}
