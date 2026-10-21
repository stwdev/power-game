package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	root := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html>")},
		"game.wasm":  {Data: []byte("\x00asm")},
		".secret":    {Data: []byte("no")},
		"sub/x.txt":  {Data: []byte("x")},
	}
	h := handler(root, false)
	cases := []struct {
		method, path string
		code         int
	}{
		{"GET", "/", 200},
		{"GET", "/index.html", 301}, // FileServer canonicalises to /
		{"GET", "/game.wasm", 200},
		{"GET", "/.secret", 404},
		{"GET", "/sub/", 404},
		{"GET", "/../../etc/passwd", 404},
		{"POST", "/", 405},
		{"HEAD", "/game.wasm", 200},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s %s: got %d want %d", c.method, c.path, rec.Code, c.code)
		}
		if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s %s: missing security headers", c.method, c.path)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Error("index should not be cached")
	}
	_ = http.StatusOK
}
