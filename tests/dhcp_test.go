package tftp_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/opnlaas/tftp"
)

type recordingPacketConn struct {
	writes int
	data   []byte
	peer   net.Addr
}

func (r *recordingPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return 0, nil, errors.New("not implemented")
}

func (r *recordingPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	r.writes++
	r.data = append([]byte(nil), p...)
	r.peer = addr
	return len(p), nil
}

func (r *recordingPacketConn) Close() error { return nil }

func (r *recordingPacketConn) LocalAddr() net.Addr { return &net.UDPAddr{IP: net.IPv4zero, Port: 0} }

func (r *recordingPacketConn) SetDeadline(t time.Time) error      { return nil }
func (r *recordingPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (r *recordingPacketConn) SetWriteDeadline(t time.Time) error { return nil }

func TestDHCPAllowlistBlocksUnknownMAC(t *testing.T) {
	allocator := tftp.DHCPAllocatorFunc(func(req *tftp.DHCPRequest) (*tftp.DHCPOffer, error) {
		return &tftp.DHCPOffer{
			YourIP:     net.IPv4(192, 0, 2, 100),
			SubnetMask: net.IPv4Mask(255, 255, 255, 0),
		}, nil
	})

	srv, err := tftp.NewServer(tftp.Options{
		DHCPAllocator:   allocator,
		AllowedDHCPMACs: []string{"aa:bb:cc:dd:ee:ff"},
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	hw := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	req, err := dhcpv4.NewDiscovery(hw)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}

	pc := &recordingPacketConn{}
	srv.DHCPHandler(pc, &net.UDPAddr{IP: net.IPv4zero, Port: 68}, req)

	if pc.writes != 0 {
		t.Fatalf("expected no response for disallowed MAC, got %d writes", pc.writes)
	}
}

func TestDHCPAllowlistAllowsKnownMAC(t *testing.T) {
	allocator := tftp.DHCPAllocatorFunc(func(req *tftp.DHCPRequest) (*tftp.DHCPOffer, error) {
		return &tftp.DHCPOffer{
			YourIP:     net.IPv4(192, 0, 2, 101),
			SubnetMask: net.IPv4Mask(255, 255, 255, 0),
		}, nil
	})

	srv, err := tftp.NewServer(tftp.Options{
		DHCPAllocator:   allocator,
		AllowedDHCPMACs: []string{"00:11:22:33:44:55"},
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	hw := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	req, err := dhcpv4.NewDiscovery(hw)
	if err != nil {
		t.Fatalf("NewDiscovery failed: %v", err)
	}

	pc := &recordingPacketConn{}
	srv.DHCPHandler(pc, &net.UDPAddr{IP: net.IPv4zero, Port: 68}, req)

	if pc.writes != 1 {
		t.Fatalf("expected one response for allowed MAC, got %d", pc.writes)
	}
	if len(pc.data) == 0 {
		t.Fatalf("expected response payload to be recorded")
	}
}
