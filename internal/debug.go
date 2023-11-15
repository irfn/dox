package dox

import (
	"log"
	"net/http"
	"net/http/httputil"
)

type DebugTransport struct{}

func (s *DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	bytes, _ := httputil.DumpRequestOut(r, true)

	resp, err := http.DefaultTransport.RoundTrip(r)
	// err is returned after dumping the response

	respBytes, _ := httputil.DumpResponse(resp, true)
	bytes = append(bytes, respBytes...)

	log.Printf("%s\n", bytes)

	return resp, err
}
