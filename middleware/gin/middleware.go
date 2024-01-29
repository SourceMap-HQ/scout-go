package gin

import (
	"github.com/scout-inc/scout-go"
	"go.opentelemetry.io/otel/attribute"

	"github.com/gin-gonic/gin"

	"github.com/scout-inc/scout-go/middleware"
)

// gin-compatible middleware
func Middleware() gin.HandlerFunc {
	middleware.AssertScoutIsRunning()

	return func(c *gin.Context) {
		requestDetails := c.GetHeader(scout.RequestTracerHeader)
		secureSessionId, requestId, err := scout.ExtractIdsFromRequest(requestDetails)
		if err != nil {
			return
		}

		c.Set(string(scout.ContextKeys.SessionSecureID), secureSessionId)
		c.Set(string(scout.ContextKeys.RequestID), requestId)

		span, _ := scout.StartTrace(c, scout.ScopedKey("gin", nil))
		defer scout.EndTrace(span)

		c.Next()

		span.SetAttributes(attribute.String(scout.SourceAttribute, "GoGinMiddleware"))
		span.SetAttributes(middleware.GetRequestAttributes(c.Request)...)
		if len(c.Errors) > 0 {
			scout.RecordSpanError(span, c.Errors[0])
		}
	}
}
