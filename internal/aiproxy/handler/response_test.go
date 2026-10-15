package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type responseBody struct {
	reader io.Reader
	closes int
	onEOF  func()
}

func TestWriteModelResponsePreservesStatusAndEndToEndHeaders(t *testing.T) {
	body := &responseBody{reader: strings.NewReader(`{"error":"provider limit"}`)}
	headers := http.Header{
		"Content-Type": {"application/json"}, "X-Provider": {"original"},
		"Connection": {"keep-alive, X-Private-Hop"}, "X-Private-Hop": {"private"},
		"Proxy-Connection": {"keep-alive"}, "Keep-Alive": {"timeout=1"},
		"Proxy-Authenticate": {"private"}, "Proxy-Authorization": {"private"},
		"Te": {"trailers"}, "Trailer": {"X-Internal"}, "Transfer-Encoding": {"chunked"}, "Upgrade": {"private"},
	}
	before := headers.Clone()
	upstream := &http.Response{StatusCode: http.StatusTooManyRequests, Header: headers, Body: body, ContentLength: 26}
	response := httptest.NewRecorder()

	writeModelResponse(t.Context(), response, upstream, slog.Default())

	if response.Code != 429 || response.Body.String() != `{"error":"provider limit"}` || body.closes != 1 {
		t.Errorf("writeModelResponse(provider error) = (%d, %s, closes %d), want original response and one close", response.Code, response.Body, body.closes)
	}
	want := http.Header{"Content-Type": {"application/json"}, "X-Provider": {"original"}}
	if diff := cmp.Diff(want, response.Header()); diff != "" {
		t.Errorf("writeModelResponse() headers mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(before, upstream.Header); diff != "" {
		t.Errorf("writeModelResponse() modified source headers (-want +got):\n%s", diff)
	}
}

func TestWriteModelResponseFlushesSSE(t *testing.T) {
	upstream := modelResponse(http.StatusOK, "data: first\n\ndata: [DONE]\n\n", http.Header{"Content-Type": {"text/event-stream; charset=utf-8"}})
	response := httptest.NewRecorder()

	writeModelResponse(t.Context(), response, upstream, slog.Default())

	if !response.Flushed || response.Body.String() != "data: first\n\ndata: [DONE]\n\n" {
		t.Errorf("writeModelResponse(SSE) flushed/body = %t/%q, want flushed unchanged events", response.Flushed, response.Body.String())
	}
}

func TestWriteModelResponseCopiesTrailersAfterEOF(t *testing.T) {
	trailers := http.Header{"X-Checksum": nil}
	body := &responseBody{reader: strings.NewReader("response"), onEOF: func() {
		trailers.Set("X-Checksum", "verified")
		trailers.Set("X-Late", "late value")
	}}
	upstream := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Trailer: trailers, Body: body, ContentLength: -1}
	response := httptest.NewRecorder()

	writeModelResponse(t.Context(), response, upstream, slog.Default())

	result := response.Result()
	defer func() {
		if err := result.Body.Close(); err != nil {
			t.Errorf("Close(recorder result) error = %v", err)
		}
	}()
	if result.Trailer.Get("X-Checksum") != "verified" || result.Trailer.Get("X-Late") != "late value" || body.closes != 1 {
		t.Errorf("writeModelResponse(trailers) = (%v, closes %d), want populated announced/late trailers and one close", result.Trailer, body.closes)
	}
}

func TestWriteModelResponseAbortsPartialBody(t *testing.T) {
	cause := errors.New("interrupted upstream stream")
	body := &responseBody{reader: io.MultiReader(strings.NewReader("data: first\n\n"), &failingBody{err: cause})}
	upstream := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, ContentLength: -1}
	response := httptest.NewRecorder()
	var interrupted any

	func() {
		defer func() { interrupted = recover() }()
		writeModelResponse(t.Context(), response, upstream, slog.Default())
	}()

	if interrupted != http.ErrAbortHandler || body.closes != 1 || response.Body.String() != "data: first\n\n" {
		t.Errorf("writeModelResponse(interrupted) panic/closes/body = %v/%d/%q, want HTTP abort, one close and no appended JSON", interrupted, body.closes, response.Body.String())
	}
}

func (b *responseBody) Read(data []byte) (int, error) {
	n, err := b.reader.Read(data)
	if errors.Is(err, io.EOF) && b.onEOF != nil {
		b.onEOF()
	}

	return n, err
}

func (b *responseBody) Close() error {
	b.closes++

	return nil
}
