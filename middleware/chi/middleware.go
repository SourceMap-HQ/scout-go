package chi

import (
	"net/http"

	"github.com/scout-inc/scout-go"
	"github.com/scout-inc/scout-go/middleware"

	"go.opentelemetry.io/otel/attribute"
)

// chi-compatible middleware
func Middleware(next http.Handler) http.Handler {
	middleware.AssertScoutIsRunning()

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := scout.InterceptRequest(r)
		span, ctx := scout.StartTrace(ctx, scout.ScopedKey("chi", nil))
		defer scout.EndTrace(span)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)

		span.SetAttributes(attribute.String(scout.SourceAttribute, "GoChiMiddleware"))
		span.SetAttributes(middleware.GetRequestAttributes(r)...)
	}
	return http.HandlerFunc(fn)
}
