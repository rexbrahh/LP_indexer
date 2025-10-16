package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	server "github.com/nats-io/nats-server/v2/server"
	nats "github.com/nats-io/nats.go"
)

func TestServiceBridgesMessages(t *testing.T) {
	srcSrv, srcURL := runJetStream(t)
	defer srcSrv.Shutdown()
	ensureStream(t, srcURL, "DEX", []string{"dex.sol.swap.*"})

	tgtSrv, tgtURL := runJetStream(t)
	defer tgtSrv.Shutdown()
	ensureStream(t, tgtURL, "legacy", []string{"legacy.swap.*"})

	svc, err := New(Config{
		SourceURL:    srcURL,
		TargetURL:    tgtURL,
		SourceStream: "DEX",
		TargetStream: "legacy",
		SubjectMappings: []SubjectMapping{
			{Source: "dex.sol.swap.", Target: "legacy.swap."},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.Run(ctx)
	}()

	srcJS := jetStreamContext(t, srcURL)
	header := nats.Header{}
	header.Set("Nats-Msg-Id", "501:123:sig123:0")
	if _, err := srcJS.PublishMsg(&nats.Msg{
		Subject: "dex.sol.swap.raydium",
		Header:  header,
		Data:    []byte("payload"),
	}, nats.ExpectStream("DEX")); err != nil {
		t.Fatalf("publish to source: %v", err)
	}

	tgtJS := jetStreamContext(t, tgtURL)
	waitFor(t, 5*time.Second, func() error {
		msg, err := tgtJS.GetLastMsg("legacy", "legacy.swap.raydium")
		if err != nil {
			return err
		}
		if string(msg.Data) != "payload" {
			return fmt.Errorf("unexpected payload %q", msg.Data)
		}
		if got := msg.Header.Get("Nats-Msg-Id"); got != "501:123:sig123:0" {
			return fmt.Errorf("unexpected msg id %q", got)
		}
		return nil
	})

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not exit after cancellation")
	}
}

func runJetStream(t *testing.T) (*server.Server, string) {
	t.Helper()
	opts := &server.Options{JetStream: true, Host: "127.0.0.1", Port: -1, StoreDir: t.TempDir()}
	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		t.Skip("nats-server not ready in sandbox")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if nc, err := nats.Connect(srv.ClientURL()); err == nil {
			nc.Close()
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	addr := srv.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		srv.Shutdown()
		t.Fatal("unexpected address type")
	}
	if tcpAddr.Port == 0 {
		srv.Shutdown()
		t.Skip("nats server no port in sandbox")
	}
	url := fmt.Sprintf("nats://127.0.0.1:%d", tcpAddr.Port)
	return srv, url
}

func ensureStream(t *testing.T, url, stream string, subjects []string) {
	t.Helper()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect ensure stream: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream ensure stream: %v", err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: stream, Subjects: subjects, Storage: nats.MemoryStorage}); err != nil {
		t.Fatalf("add stream: %v", err)
	}
}

func jetStreamContext(t *testing.T, url string) nats.JetStreamContext {
	t.Helper()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect jetstream: %v", err)
	}
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream ctx: %v", err)
	}
	return js
}

func waitFor(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := fn(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("condition not met before timeout")
	}
	t.Fatalf("waitFor: %v", lastErr)
}
