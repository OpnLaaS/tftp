package tftp

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

type (
	GetType uint8

	Options struct {
		ListenAddrTFTP, ListenAddrHTTP string
		Getter                         Getter

		ListenAddrDHCP string
		DHCPAllocator  DHCPAllocator
		DHCPServerIP   net.IP
		AllowedDHCPMACs []string
	}

	Server struct {
		Options     Options
		QuitChannel chan struct{}

		getter   Getter
		getterMu sync.RWMutex

		ctx    context.Context
		cancel context.CancelFunc

		httpServer *http.Server
		tftpConn   *net.UDPConn
		dhcpServer *server4.Server

		dhcpMACMu       sync.RWMutex
		dhcpAllowedMACs map[string]struct{}

		wg sync.WaitGroup
	}

	Requestor struct {
		IPAddress  *string
		MacAddress *string
	}

	Context struct {
		GetType  GetType
		Filename string
		From     *Requestor
	}

	Getter interface {
		Get(getType GetType, ctx *Context) ([]byte, error)
	}
)

type GetterFunc func(getType GetType, ctx *Context) ([]byte, error)

func (f GetterFunc) Get(getType GetType, ctx *Context) ([]byte, error) {
	return f(getType, ctx)
}
