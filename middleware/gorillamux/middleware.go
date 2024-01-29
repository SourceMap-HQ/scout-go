package gorillamux

import (
	"net/http"

	"github.com/scout-inc/scout-go/middleware"
	"go.opentelemetry.io/otel/attribute"

	"github.com/scout-inc/scout-go"
)

// gorilla-compatible middleware
func Middleware(next http.Handler) http.Handler {
	middleware.AssertScoutIsRunning()

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := scout.InterceptRequest(r)
		r = r.WithContext(ctx)

		span, ctx := scout.StartTrace(ctx, scout.ScopedKey("gorillamux", nil))
		defer scout.EndTrace(span)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)

		span.SetAttributes(attribute.String(scout.SourceAttribute, "GoGorillaMuxMiddleware"))
		span.SetAttributes(middleware.GetRequestAttributes(r)...)
	}
	return http.HandlerFunc(fn)
}
