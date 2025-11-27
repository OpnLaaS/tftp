package tftp_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opnlaas/tftp"
)

func TestHTTPHandlerUsesGetter(t *testing.T) {
	seenCtx := make(chan *tftp.Context, 1)
	getter := tftp.GetterFunc(func(gt tftp.GetType, ctx *tftp.Context) ([]byte, error) {
		seenCtx <- ctx
		return []byte("abc"), nil
	})

	s, err := tftp.NewServer(tftp.Options{Getter: getter})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/boot.ipxe?mac=aa:bb:cc:dd:ee:ff", nil)
	req.RemoteAddr = net.JoinHostPort("192.0.2.10", "1234")
	rr := httptest.NewRecorder()

	s.HTTPHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d", rr.Code)
	}

	body := rr.Body.String()
	if body != "abc" {
		t.Fatalf("unexpected body: %q", body)
	}

	ctx := <-seenCtx
	if ctx.GetType != tftp.GetTypeHTTP {
		t.Fatalf("expected GetTypeHTTP, got %v", ctx.GetType)
	}

	if ctx.Filename != "boot.ipxe" {
		t.Fatalf("unexpected filename: %s", ctx.Filename)
	}

	if ctx.From == nil || ctx.From.IPAddress == nil || *ctx.From.IPAddress != "192.0.2.10" {
		t.Fatalf("expected requester IP to be captured, got %#v", ctx.From)
	}

	if ctx.From.MacAddress == nil || *ctx.From.MacAddress != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected mac address to be captured, got %#v", ctx.From.MacAddress)
	}
}

func TestGetWithoutGetterErrors(t *testing.T) {
	s := &tftp.Server{}
	if _, err := s.Get(tftp.GetTypeHTTP, &tftp.Context{}); err == nil {
		t.Fatalf("expected error when getter is nil")
	}
}
