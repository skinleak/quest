package engine

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/skinleak/quest/internal/core/request"
)

// tracer records the phase boundaries of a request. net/http calls the
// hooks from its own goroutines (DNS and dialing can race), hence the lock.
type tracer struct {
	mu                       sync.Mutex
	start                    time.Time
	dnsStart, dnsDone        time.Time
	connectStart, connectEnd time.Time
	tlsStart, tlsDone        time.Time
	wrote, firstByte         time.Time
	reused                   bool
	remote                   string
}

func (t *tracer) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		// GetConn starts every hop, so a redirect's phases replace the
		// previous hop's instead of mixing with them.
		GetConn: func(string) {
			t.mu.Lock()
			t.dnsStart, t.dnsDone = time.Time{}, time.Time{}
			t.connectStart, t.connectEnd = time.Time{}, time.Time{}
			t.tlsStart, t.tlsDone = time.Time{}, time.Time{}
			t.wrote, t.firstByte = time.Time{}, time.Time{}
			t.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.reused = info.Reused
			if info.Conn != nil {
				t.remote = info.Conn.RemoteAddr().String()
			}
			t.mu.Unlock()
		},
		DNSStart: func(httptrace.DNSStartInfo) { t.mark(&t.dnsStart, false) },
		DNSDone:  func(httptrace.DNSDoneInfo) { t.mark(&t.dnsDone, false) },
		// Happy Eyeballs dials IPv4 and IPv6 in parallel and reports each
		// attempt: the phase runs from the first start to the first success.
		ConnectStart: func(string, string) { t.mark(&t.connectStart, true) },
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				t.mark(&t.connectEnd, true)
			}
		},
		TLSHandshakeStart:    func() { t.mark(&t.tlsStart, false) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { t.mark(&t.tlsDone, false) },
		WroteRequest:         func(httptrace.WroteRequestInfo) { t.mark(&t.wrote, false) },
		GotFirstResponseByte: func() { t.mark(&t.firstByte, false) },
	}
}

// mark sets *at to now; with once, only if it is not set yet.
func (t *tracer) mark(at *time.Time, once bool) {
	now := time.Now()
	t.mu.Lock()
	if !once || at.IsZero() {
		*at = now
	}
	t.mu.Unlock()
}

// remoteAddr is the address of the last hop's server.
func (t *tracer) remoteAddr() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.remote
}

// timing turns the marks into durations, with end as the end of the body.
func (t *tracer) timing(end time.Time) request.Timing {
	t.mu.Lock()
	defer t.mu.Unlock()
	return request.Timing{
		DNS:      between(t.dnsStart, t.dnsDone),
		Connect:  between(t.connectStart, t.connectEnd),
		TLS:      between(t.tlsStart, t.tlsDone),
		TTFB:     between(t.wrote, t.firstByte),
		Transfer: between(t.firstByte, end),
		Total:    between(t.start, end),
		Reused:   t.reused,
	}
}

func between(a, b time.Time) time.Duration {
	if a.IsZero() || b.IsZero() || b.Before(a) {
		return 0
	}
	return b.Sub(a)
}
