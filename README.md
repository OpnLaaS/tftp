# PXE helper (TFTP + HTTP + DHCP)

Lightweight PXE building block that:
- Serves boot artifacts over TFTP and HTTP with per-request customization via a pluggable `Getter`.
- (Optional) Runs a DHCP server using `insomniacslk/dhcp`, with an allowlist to avoid hijacking unintended clients.

## Install

```bash
go get github.com/opnlaas/tftp
```

## Core concepts

- `Getter`: your callback to supply bytes for either TFTP or HTTP. You decide what to serve based on filename, caller IP, or MAC.
- `Context`: passed to `Getter`, includes `GetType` (TFTP/HTTP), requested `Filename`, and `From` with IP and optional MAC (HTTP only, or injected by you).
- `Server`: wraps TFTP/HTTP (and optional DHCP) listeners with `Start`/`Stop`. You can hot-swap the getter via `SetGetter`.

## Minimal usage

```go
srv, _ := tftp.NewServer(tftp.Options{
	ListenAddrTFTP: ":6969", // UDP
	ListenAddrHTTP: ":8080", // TCP
	Getter: tftp.GetterFunc(func(gt tftp.GetType, ctx *tftp.Context) ([]byte, error) {
		if gt == tftp.GetTypeTFTP && ctx.Filename == "pxelinux.0" {
			return []byte("...binary..."), nil
		}
		if gt == tftp.GetTypeHTTP && ctx.Filename == "boot.ipxe" {
			return []byte("#!ipxe\nchain http://192.0.2.1/primary.ipxe"), nil
		}
		return nil, fmt.Errorf("not found")
	}),
})

_ = srv.Start()
defer srv.Stop()
select {} // block as needed
```

## Request metadata

- IP: always set (UDP source for TFTP, `RemoteAddr` for HTTP).
- MAC: only set automatically for HTTP (`X-Mac-Address` header or `mac` query). TFTP RRQ doesn’t carry MAC; if you need MAC-aware TFTP decisions, inject a mapping (e.g., from DHCP leases) inside your getter.

## DHCP server (optional, allowlisted)

Enable by providing DHCP options and an allocator. Use `AllowedDHCPMACs` to ensure only known hosts get leases/boot params.

```go
leases := map[string]tftp.DHCPOffer{
	"192.0.2.10": {
		YourIP:     net.IPv4(192, 0, 2, 10),
		SubnetMask: net.IPv4Mask(255, 255, 255, 0),
		Router:     net.IPv4(192, 0, 2, 1),
		DNSServers: []net.IP{net.IPv4(192, 0, 2, 53)},
		BootFile:   "pxelinux.0",
		NextServer: net.IPv4(192, 0, 2, 1),
		LeaseTime:  2 * time.Hour,
	},
}

allocator := tftp.DHCPAllocatorFunc(func(req *tftp.DHCPRequest) (*tftp.DHCPOffer, error) {
	if offer, ok := leases[req.RequestedIP.String()]; ok {
		return &offer, nil
	}
	return nil, fmt.Errorf("no lease for %s", req.RequestedIP)
})

srv, _ := tftp.NewServer(tftp.Options{
	ListenAddrDHCP:  ":67",                                // requires privileges/capabilities
	DHCPAllocator:   allocator,
	DHCPServerIP:    net.IPv4(192, 0, 2, 1),
	AllowedDHCPMACs: []string{"aa:bb:cc:dd:ee:ff"},        // empty = allow all
	ListenAddrTFTP:  ":6969",
	ListenAddrHTTP:  ":8080",
	Getter:          myGetter,
})
_ = srv.Start()
```

Notes:
- The DHCP allocator runs on DISCOVER/REQUEST with MAC, requested IP, and gateway info. Return a `DHCPOffer` with IP/netmask/router/DNS/bootfile/next-server/lease.
- DHCP on :67 typically needs privileges; use setcap or run with the right permissions.
- MAC allowlist is enforced before allocation to avoid interfering with the rest of the network.

## Swapping behavior at runtime

- `srv.SetGetter(newGetter)` to change what’s served.
- `srv.SetAllowedDHCPMACs([]string{...})` to update the DHCP allowlist on the fly.

## Testing

`go test ./...` (local sockets only). Tests cover:
- TFTP RRQ parsing and data/ACK flow (including zero-length content).
- HTTP handler context plumbing and nil-getter handling.
- DHCP allowlist enforcement vs allowed MACs.

