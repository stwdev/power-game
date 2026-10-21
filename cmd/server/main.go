// Command server serves the built game (the dist directory) with strict
// security headers. It only serves files: there are no APIs, uploads or
// directory listings. It is meant for local testing or for playing on a
// phone over your home network; for public hosting use HTTPS (-cert/-key
// or a static host such as GitHub Pages).
//
//	go run ./cmd/server -dir dist -addr 127.0.0.1:8080
package main

import (
	"errors"
	"flag"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const csp = "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; worker-src 'self'; " +
	"connect-src 'self'; style-src 'self'; img-src 'self' data:; manifest-src 'self'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (use 0.0.0.0:8080 to reach it from a phone on your Wi-Fi)")
	dir := flag.String("dir", "dist", "directory with the built game")
	cert := flag.String("cert", "", "TLS certificate file (enables HTTPS)")
	key := flag.String("key", "", "TLS key file")
	flag.Parse()

	if st, err := os.Stat(*dir); err != nil || !st.IsDir() {
		log.Fatalf("%s is not a directory; run ./build.sh first", *dir)
	}
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler(os.DirFS(*dir), *cert != ""),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	log.Printf("serving %s on %s", *dir, *addr)
	var err error
	if *cert != "" {
		err = srv.ListenAndServeTLS(*cert, *key)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func handler(root fs.FS, tls bool) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if tls {
			h.Set("Strict-Transport-Security", "max-age=63072000")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Only plain files are served: no listings, no dotfiles.
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if strings.HasPrefix(path.Base(name), ".") {
			http.NotFound(w, r)
			return
		}
		st, err := fs.Stat(root, name)
		if err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		if name == "sw.js" || name == "index.html" {
			h.Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
