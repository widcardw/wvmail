package services

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeIMAPServer accepts connections and immediately sends an IMAP banner. It is
// enough for the probe, which stops before LOGIN: the reachable path only needs
// the banner to arrive.
func fakeIMAPServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if _, err := c.Write([]byte("* OK fake IMAP ready\r\n")); err != nil {
					return
				}
				// Drain whatever the probe sends; it never gets here today, but a
				// future probe step must not be the thing that hangs the test.
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
				}
			}(conn)
		}
	}()

	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}

// silentServer accepts connections and never speaks, which is the failure the
// banner deadline exists for: the socket is open, so a dial timeout alone would
// never fire.
func silentServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var mu sync.Mutex
	var held []net.Conn
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range held {
			c.Close()
		}
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			// Hold the connection open without ever sending a banner.
		}
	}()

	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}

// httpServer answers with something that is not an IMAP banner: reachable, but
// not a mailbox.
func httpServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			}(conn)
		}
	}()

	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}

func TestProbeConnectionUnconfigured(t *testing.T) {
	svc := &NetworkService{}

	t.Run("empty host", func(t *testing.T) {
		res := svc.ProbeConnection("", 993, true)
		if res.Ok {
			t.Fatal("empty host must not report success")
		}
		if res.LatencyMS != -1 {
			t.Errorf("expected latency -1 (no measurement), got %d", res.LatencyMS)
		}
		if res.Attempts != 0 {
			t.Errorf("expected no attempts against an empty host, got %d", res.Attempts)
		}
		if res.Error == "" {
			t.Error("expected an error message for the UI to show")
		}
	})

	t.Run("port zero", func(t *testing.T) {
		if res := svc.ProbeConnection("imap.example.com", 0, true); res.Ok || res.LatencyMS != -1 {
			t.Errorf("expected an unmeasured failure, got %+v", res)
		}
	})
}

func TestProbeConnectionReachable(t *testing.T) {
	host, port := fakeIMAPServer(t)

	res := (&NetworkService{}).ProbeConnection(host, port, false)
	if !res.Ok {
		t.Fatalf("expected the fake server to be reported reachable, got %+v", res)
	}
	if res.LatencyMS < 0 {
		t.Errorf("a successful probe must carry a latency, got %d", res.LatencyMS)
	}
	if res.Attempts != 1 {
		t.Errorf("a first-try success must report 1 attempt, got %d", res.Attempts)
	}
	if res.Error != "" {
		t.Errorf("a successful probe must not carry an error, got %q", res.Error)
	}
}

func TestProbeConnectionFailure(t *testing.T) {
	// Bind and close to get a port nothing is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	res := (&NetworkService{}).ProbeConnection("127.0.0.1", port, false)
	if res.Ok {
		t.Fatalf("a refused connection must not be reported reachable: %+v", res)
	}
	if res.LatencyMS != -1 {
		t.Errorf("a failed probe must report -1, not %d", res.LatencyMS)
	}
	if res.Attempts != probeAttempts {
		t.Errorf("expected %d attempts before giving up, got %d", probeAttempts, res.Attempts)
	}
	if !strings.Contains(res.Error, "connect failed") {
		t.Errorf("expected a connect error, got %q", res.Error)
	}
}

func TestProbeIMAPReachableBannerTimeout(t *testing.T) {
	host, port := silentServer(t)

	start := time.Now()
	elapsed, err := probeIMAPReachable(host, port, false, 250*time.Millisecond)
	if err == nil {
		t.Fatal("a server that never sends a banner must not be reported reachable")
	}
	if elapsed > 3*time.Second {
		t.Errorf("the banner deadline must bound the call, took %v", elapsed)
	}
	if delta := time.Since(start); delta > 3*time.Second {
		t.Errorf("probe overran its timeout: %v", delta)
	}
}

func TestProbeIMAPReachableRejectsNonIMAP(t *testing.T) {
	host, port := httpServer(t)

	if _, err := probeIMAPReachable(host, port, false, 2*time.Second); err == nil {
		t.Fatal("a port that answers with HTTP must not read as a mail server")
	} else if !strings.Contains(err.Error(), "not an IMAP server") {
		t.Errorf("expected the banner to be rejected, got %v", err)
	}
}

func TestTruncateText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "shorter than the cap", in: "dial failed", max: 160, want: "dial failed"},
		{name: "exactly the cap", in: "abcd", max: 4, want: "abcd"},
		{name: "cut and marked", in: "abcdef", max: 4, want: "abcd…"},
		{name: "zero cap", in: "abcdef", max: 0, want: ""},
		{name: "cjk is cut by rune", in: "连接失败连接失败", max: 4, want: "连接失败…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateText(tc.in, tc.max); got != tc.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}
