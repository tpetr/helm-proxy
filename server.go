package main

import (
	"log"
	"net/http"
	"flag"

	"github.com/Masterminds/httputil"
	"github.com/technosophos/helm-proxy/transcode"
)

func main() {
	addr := flag.String("listen", ":44133", "address and port to listen on")
	paddr := flag.String("proxy-addr", "localhost:44134", "tiller address to proxy to")
	flag.Parse()
	proxy := transcode.New(*paddr)
	http.HandleFunc("/", bootstrap(proxy))
	log.Printf("starting server on %s to %s", *addr, *paddr)
	http.ListenAndServe(*addr, nil)
}

func bootstrap(proxy *transcode.Proxy) http.HandlerFunc {
	api := routes(proxy)
	rslv := httputil.NewResolver(routeNames(api))

	// The main http.HandlerFunc delegates to the right route handler.
	hf := func(w http.ResponseWriter, r *http.Request) {
		path, err := rslv.Resolve(r)
		if err != nil {
			http.NotFound(w, r)
		}
		for _, rr := range api {
			if rr.path == path {
				if err := rr.handler(w, r); err != nil {
					log.Printf("error on path %q: %s", path, err)
					http.Error(w, "proxy operation failed", 500)
				}
			}
		}
	}
	return hf
}

type routeHandler func(w http.ResponseWriter, r *http.Request) error
type route struct {
	path    string
	handler routeHandler
}

func routes(proxy *transcode.Proxy) []route {
	return []route{
		// Status
		{"GET /", index},
		// List
		{"GET /v1/releases", proxy.List},
		// Get
		{"GET /v1/releases/*", proxy.Get},
		// Install
		{"POST /v1/releases", proxy.Install},
		// Upgrade
		{"POST /v1/releases/*", proxy.Upgrade},
		// Delete
		{"DELETE /v1/releases/*", proxy.Uninstall},
		// History
		{"GET /v1/releases/*/history", proxy.History},
		// Rollback
		{"POST /v1/releases/*/history/*", proxy.Rollback},
	}
}

func routeNames(r []route) []string {
	rn := make([]string, len(r))
	for i, rr := range r {
		rn[i] = rr.path
	}
	return rn
}

func index(w http.ResponseWriter, r *http.Request) error {
	_, err := w.Write([]byte(`{status: "ok", versions:["v1"]}`))
	return err
}
