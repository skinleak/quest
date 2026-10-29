package request

import (
	"net/http"
	"time"
)

// Response is a received response. Body holds at most the engine's body
// limit; Truncated says whether more was sent.
type Response struct {
	Status      string // e.g. "200 OK"
	StatusCode  int
	Proto       string
	Header      http.Header
	Body        []byte
	Truncated   bool
	Size        int64 // bytes of body received, after decompression
	ContentType string
	URL         string // the final URL, after redirects
	Timing      Timing
}

// Timing breaks a request down into its phases. With redirects, the phases
// are those of the last hop and Total covers all of them. Phases that did
// not happen, such as DNS on a reused connection, are zero.
type Timing struct {
	DNS      time.Duration
	Connect  time.Duration
	TLS      time.Duration
	TTFB     time.Duration // from sending the request to the first response byte
	Transfer time.Duration // from the first response byte to the end of the body
	Total    time.Duration // from start until the body was read
	Reused   bool          // the connection came from the pool
}
