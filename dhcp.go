package tftp

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

// DHCPRequest represents the parsed DHCP client request for allocator decisions.
type DHCPRequest struct {
	MessageType dhcpv4.MessageType
	XID         uint32
	ClientMAC   net.HardwareAddr
	RequestedIP net.IP
	CurrentIP   net.IP
	GatewayIP   net.IP
}

// DHCPOffer describes the parameters the server will offer/ack to a client.
type DHCPOffer struct {
	YourIP     net.IP
	SubnetMask net.IPMask
	Router     net.IP
	DNSServers []net.IP
	DomainName string
	BootFile   string
	NextServer net.IP
	LeaseTime  time.Duration
}

// DHCPAllocator decides what to offer to a DHCP client.
type DHCPAllocator interface {
	Offer(req *DHCPRequest) (*DHCPOffer, error)
}

type DHCPAllocatorFunc func(req *DHCPRequest) (*DHCPOffer, error)

func (f DHCPAllocatorFunc) Offer(req *DHCPRequest) (*DHCPOffer, error) {
	return f(req)
}

func (s *Server) startDHCP(ctx context.Context) error {
	if s.Options.DHCPAllocator == nil || s.Options.ListenAddrDHCP == "" {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp4", s.Options.ListenAddrDHCP)
	if err != nil {
		return err
	}

	server, err := server4.NewServer("", addr, s.dhcpHandler, server4.WithSummaryLogger())
	if err != nil {
		return err
	}

	s.dhcpServer = server
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		_ = server.Serve()
	}()

	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	return nil
}

func (s *Server) dhcpHandler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	if !s.dhcpMACAllowed(m.ClientHWAddr) {
		return
	}

	req := &DHCPRequest{
		MessageType: m.MessageType(),
		XID:         binary.BigEndian.Uint32(m.TransactionID[:]),
		ClientMAC:   m.ClientHWAddr,
		RequestedIP: m.RequestedIPAddress(),
		CurrentIP:   m.ClientIPAddr,
		GatewayIP:   m.GatewayIPAddr,
	}

	offer, err := s.Options.DHCPAllocator.Offer(req)
	if err != nil || offer == nil {
		return
	}

	msgType := dhcpv4.MessageTypeOffer
	if req.MessageType == dhcpv4.MessageTypeRequest {
		msgType = dhcpv4.MessageTypeAck
	}

	serverIP := s.Options.DHCPServerIP
	if serverIP == nil {
		if host, _, err := net.SplitHostPort(s.Options.ListenAddrDHCP); err == nil {
			serverIP = net.ParseIP(host)
		}
	}
	if serverIP == nil {
		serverIP = net.IPv4zero
	}

	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(msgType),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(serverIP)),
		dhcpv4.WithYourIP(offer.YourIP),
	}

	if offer.SubnetMask != nil {
		modifiers = append(modifiers, dhcpv4.WithNetmask(offer.SubnetMask))
	}
	if offer.Router != nil {
		modifiers = append(modifiers, dhcpv4.WithRouter(offer.Router))
	}
	if len(offer.DNSServers) > 0 {
		modifiers = append(modifiers, dhcpv4.WithDNS(offer.DNSServers...))
	}
	if offer.DomainName != "" {
		modifiers = append(modifiers, dhcpv4.WithOption(dhcpv4.OptDomainName(offer.DomainName)))
	}
	if offer.LeaseTime > 0 {
		modifiers = append(modifiers, dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(offer.LeaseTime)))
	}
	if offer.NextServer != nil {
		modifiers = append(modifiers, dhcpv4.WithOption(dhcpv4.OptTFTPServerName(offer.NextServer.String())))
	}
	if offer.BootFile != "" {
		modifiers = append(modifiers, dhcpv4.WithOption(dhcpv4.OptBootFileName(offer.BootFile)))
	}

	resp, err := dhcpv4.NewReplyFromRequest(m, modifiers...)
	if err != nil {
		return
	}

	raw := resp.ToBytes()
	_, _ = conn.WriteTo(raw, peer)
}

// DHCPHandler exposes the DHCP handler for testing or embedding.
func (s *Server) DHCPHandler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	s.dhcpHandler(conn, peer, m)
}

func (s *Server) dhcpMACAllowed(hw net.HardwareAddr) bool {
	s.dhcpMACMu.RLock()
	allowed := s.dhcpAllowedMACs
	s.dhcpMACMu.RUnlock()

	if len(allowed) == 0 {
		return true
	}

	norm, ok := normalizeMACString(hw.String())
	if !ok {
		return false
	}
	_, ok = allowed[norm]
	return ok
}

func normalizeMACString(mac string) (string, bool) {
	parsed, err := net.ParseMAC(strings.ReplaceAll(mac, "-", ":"))
	if err != nil {
		return "", false
	}
	return strings.ToLower(parsed.String()), true
}

func normalizeMACList(macStrs []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, m := range macStrs {
		if norm, ok := normalizeMACString(m); ok {
			out[norm] = struct{}{}
		}
	}
	return out
}
