package tftp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
)

var errNoGetterConfigured = errors.New("no getter configured")

// NewServer configures a PXE helper with both TFTP and HTTP frontends.
func NewServer(options Options) (*Server, error) {
	server := &Server{
		Options:     options,
		QuitChannel: make(chan struct{}),
		getter:      options.Getter,
	}

	server.SetAllowedDHCPMACs(options.AllowedDHCPMACs)

	return server, nil
}

// SetGetter swaps the active getter at runtime so callers can customize content per requestor.
func (s *Server) SetGetter(getter Getter) {
	s.getterMu.Lock()
	defer s.getterMu.Unlock()
	s.getter = getter
}

// SetAllowedDHCPMACs replaces the DHCP allowlist (empty = allow all).
func (s *Server) SetAllowedDHCPMACs(macs []string) {
	s.dhcpMACMu.Lock()
	defer s.dhcpMACMu.Unlock()
	s.dhcpAllowedMACs = normalizeMACList(macs)
}

// Get proxies to the configured getter while safely handling the nil case.
func (s *Server) Get(getType GetType, ctx *Context) ([]byte, error) {
	s.getterMu.RLock()
	getter := s.getter
	s.getterMu.RUnlock()

	if getter == nil {
		return nil, errNoGetterConfigured
	}

	return getter.Get(getType, ctx)
}

// Start brings up both TFTP and HTTP listeners. It returns an error if either listener
// cannot bind.
func (s *Server) Start() error {
	if s.cancel != nil {
		return errors.New("server already started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	if err := s.startTFTP(ctx); err != nil {
		cancel()
		return fmt.Errorf("start tftp: %w", err)
	}

	if err := s.startHTTP(ctx); err != nil {
		cancel()
		return fmt.Errorf("start http: %w", err)
	}

	if err := s.startDHCP(ctx); err != nil {
		cancel()
		return fmt.Errorf("start dhcp: %w", err)
	}

	return nil
}

// Stop shuts down listeners and waits for handlers to drain.
func (s *Server) Stop() {
	select {
	case <-s.QuitChannel:
	default:
		close(s.QuitChannel)
	}

	if s.cancel != nil {
		s.cancel()
	}

	if s.tftpConn != nil {
		_ = s.tftpConn.Close()
	}

	if s.dhcpServer != nil {
		_ = s.dhcpServer.Close()
	}

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(ctx)
	}

	s.wg.Wait()
}

func (s *Server) startTFTP(ctx context.Context) error {
	if s.Options.ListenAddrTFTP == "" {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp4", s.Options.ListenAddrTFTP)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}

	s.tftpConn = conn
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		defer conn.Close()

		buf := make([]byte, 1500)

		for {
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}
				continue
			}

			payload := append([]byte(nil), buf[:n]...)
			go s.handleTFTPRequest(ctx, conn, clientAddr, payload)
		}
	}()

	return nil
}

func (s *Server) handleTFTPRequest(ctx context.Context, conn *net.UDPConn, clientAddr *net.UDPAddr, payload []byte) {
	if len(payload) < 2 {
		return
	}

	if payload[1] != OPCODE_RRQ {
		_ = sendErrorTFTP(conn, clientAddr, 4, "Unsupported operation")
		return
	}

	filename, mode, err := ParseRRQRequestTFTP(payload)
	if err != nil {
		_ = sendErrorTFTP(conn, clientAddr, 0, "Invalid request")
		return
	}

	_ = mode // reserved for future use; currently accepts anything

	from := &Requestor{}
	ip := clientAddr.IP.String()
	from.IPAddress = &ip

	content, err := s.Get(GetTypeTFTP, &Context{
		GetType:  GetTypeTFTP,
		Filename: filename,
		From:     from,
	})
	if err != nil {
		_ = sendErrorTFTP(conn, clientAddr, 1, err.Error())
		return
	}

	dataConn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		_ = sendErrorTFTP(conn, clientAddr, 0, "unable to open data socket")
		return
	}
	defer dataConn.Close()

	_ = SendBufferTFTP(ctx, dataConn, clientAddr, content)
}

func (s *Server) startHTTP(ctx context.Context) error {
	if s.Options.ListenAddrHTTP == "" {
		return nil
	}

	ln, err := net.Listen("tcp", s.Options.ListenAddrHTTP)
	if err != nil {
		return err
	}

	s.httpServer = &http.Server{
		Addr:    s.Options.ListenAddrHTTP,
		Handler: s.HTTPHandler(),
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// nothing to do here other than exit; callers will see shutdown by Stop.
		}
	}()

	go func() {
		<-ctx.Done()
		_ = s.httpServer.Shutdown(context.Background())
	}()

	return nil
}

// HTTPHandler returns the HTTP handler used for serving dynamic artifacts.
func (s *Server) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.httpHandler)
	return mux
}

func (s *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if filename == "" || filename == "." {
		http.NotFound(w, r)
		return
	}

	req := &Requestor{}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		req.IPAddress = &ip
	}

	if mac := r.Header.Get("X-Mac-Address"); mac != "" {
		req.MacAddress = &mac
	} else if mac := r.URL.Query().Get("mac"); mac != "" {
		req.MacAddress = &mac
	}

	content, err := s.Get(GetTypeHTTP, &Context{
		GetType:  GetTypeHTTP,
		Filename: filename,
		From:     req,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(content)
}
