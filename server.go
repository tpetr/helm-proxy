package main

import (
	"log"
	"net/http"
	"flag"

	"github.com/Masterminds/httputil"
	"github.com/technosophos/helm-proxy/transcode"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/pkg/apis/authentication/v1"
	"strings"
)

func main() {
	addr := flag.String("listen", ":44133", "address and port to listen on")
	paddr := flag.String("proxy-addr", "localhost:44134", "tiller address to proxy to")
	flag.Parse()
	proxy := transcode.New(*paddr)

	auth := authenticationv1.NewForConfigOrDie(kubeConfigOrDie())

	http.HandleFunc("/", bootstrap(proxy, auth))
	log.Printf("starting server on %s to %s", *addr, *paddr)
	http.ListenAndServe(*addr, nil)
}

func kubeConfigOrDie() (*rest.Config) {
	// Try in-cluster config
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg
	}

	// Try file specified by KUBECONFIG
	if cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG")); err != nil {
		panic(err)
	} else {
		return cfg
	}
}

func bootstrap(proxy *transcode.Proxy, auth *authenticationv1.AuthenticationV1Client) http.HandlerFunc {
	api := routes(proxy)
	rslv := httputil.NewResolver(routeNames(api))

	tokenReviews := auth.TokenReviews()

	// The main http.HandlerFunc delegates to the right route handler.
	hf := func(w http.ResponseWriter, r *http.Request) {
		path, err := rslv.Resolve(r)
		if err != nil {
			http.NotFound(w, r)
		}

		authHeaderValue := r.Header.Get("Authorization")

		if !strings.HasPrefix(authHeaderValue, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		result, err := tokenReviews.Create(&v1.TokenReview{
			Spec: v1.TokenReviewSpec{
				Token: authHeaderValue[7:],
			},
		})

		if err != nil {
			log.Printf("Error validating token: %s", err)
			http.Error(w, "Token validation failed", http.StatusInternalServerError)
			return
		}

		if result.Status.Error != "" {
			log.Printf("Error validating token: %s", result.Status.Error)
			http.Error(w, "Token validation failed", http.StatusInternalServerError)
			return
		}

		if !result.Status.Authenticated {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
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
