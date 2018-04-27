package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	_ "github.com/joho/godotenv/autoload"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	things "github.com/aodin/grpc/go"
)

var bindAddress = ":8080"

// newGateway returns a new gateway server which translates HTTP into gRPC.
func newGateway(ctx context.Context, opts ...runtime.ServeMuxOption) (http.Handler, error) {
	log.Printf("starting gateway server on %s\n", bindAddress)
	mux := runtime.NewServeMux(opts...)
	dialOpts := []grpc.DialOption{grpc.WithInsecure()}

	if err := things.RegisterThingsHandlerFromEndpoint(ctx, mux, ":10808", dialOpts); err != nil {
		return nil, err
	}
	return mux, nil
}

// handleCORS allows Cross Origin Resoruce Sharing from any origin.
// Don't do this without consideration in production systems.
func handleCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
				preflightHandler(w, r)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// loggingResponseWriter records the status code of the response
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// redirectHTTP will redirect any requests with a value of 'http' in the
// header 'X-Forwarded-Proto' to HTTPS
func redirectHTTP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			// Use RequestURI since it is what the client set, unmodified
			// Other methods, such as URL.String(), reassemble the URL
			redirectTo := "https://" + r.Host + r.RequestURI
			http.Redirect(w, r, redirectTo, http.StatusMovedPermanently)
			return
		}
		// Continue the request as usual if there is any other value or no
		// value at all
		h.ServeHTTP(w, r)
	})
}

// headers is allowed CORS headers
var headers = []string{
	"Accept",
	"Accept-Encoding",
	"Authorization",
	"Content-Type",
	"Origin",
	"User-Agent",
	"X-CSRF-Token",
	"X-CSRFToken",
	"X-Request-ID",
	"X-Requested-With",
}

// methods are the HTTP methods allowed by CORS
var methods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

// preflightHandler sets headers values for CORS and then short-circuits
// the request
func preflightHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ","))
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ","))
	w.Header().Set("Access-Control-Max-Age", "3600") // One hour
	return
}

// Run starts a HTTP server and blocks forever if successful.
func Run(address string, opts ...runtime.ServeMuxOption) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := http.NewServeMux()
	gw, err := newGateway(ctx, opts...)
	if err != nil {
		return err
	}
	mux.Handle("/", gw)
	return http.ListenAndServe(
		address,
		handleCORS(mux),
	)
}

func main() {
	log.SetFlags(0) // Remove timestamp - it will be set by log ingestion
	if err := Run(bindAddress); err != nil {
		log.Fatal(err)
	}
}