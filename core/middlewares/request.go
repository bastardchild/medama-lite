package middlewares

import (
	"context"
	"net/http"

	"github.com/medama-io/medama/model"
	"github.com/ogen-go/ogen/middleware"
)

// RequestContext adds the request object to the context for use in handlers.
func RequestContext() middleware.Middleware {
	return func(
		req middleware.Request,
		next func(req middleware.Request) (middleware.Response, error),
	) (middleware.Response, error) {
		// Match with event operations
		switch req.OperationID {
		case "post-event-hit", "get-event-ping":
			// Add the request to the context
			req.Context = context.WithValue(req.Context, model.RequestKeyBody, req.Raw)
		}

		return next(req)
	}
}

// EventContentTypeMiddleware ensures that Beacon API requests (which browsers send with
// text/plain Content-Type or no Content-Type) are recognized and parsed as application/json.
func EventContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && (r.URL.Path == "/event/hit" || r.URL.Path == "/event/hit/" || r.URL.Path == "/api/event/hit") {
			ct := r.Header.Get("Content-Type")
			if ct == "" || ct == "text/plain" || len(ct) >= 10 && ct[:10] == "text/plain" {
				r.Header.Set("Content-Type", "application/json")
			}
		}
		next.ServeHTTP(w, r)
	})
}
