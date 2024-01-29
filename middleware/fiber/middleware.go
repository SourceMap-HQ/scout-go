package fiber

import (
	"github.com/gofiber/fiber/v2"
	"github.com/scout-inc/scout-go"
	"github.com/scout-inc/scout-go/middleware"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// fiber-compatible middleware
func Middleware() fiber.Handler {
	middleware.AssertScoutIsRunning()

	return func(c *fiber.Ctx) error {
		ctx := c.Context()

		requestDetails := string(c.Request().Header.Peek(scout.RequestTracerHeader))
		sessionSecureId, requestId, err := scout.ExtractIdsFromRequest(requestDetails)
		if err == nil {
			ctx.SetUserValue(scout.ContextKeys.SessionSecureID, sessionSecureId)
			ctx.SetUserValue(scout.ContextKeys.RequestID, requestId)
		}

		span, scoutContext := scout.StartTrace(ctx, scout.ScopedKey("fiber", nil))
		defer scout.EndTrace(span)

		c.SetUserContext(scoutContext)
		err = c.Next()

		scout.RecordSpanError(
			span, err,
			attribute.String(scout.SourceAttribute, "GoFiberMiddleware"),
			attribute.String(string(semconv.HTTPURLKey), c.OriginalURL()),
			attribute.String(string(semconv.HTTPRouteKey), c.Path()),
			attribute.String(string(semconv.HTTPMethodKey), c.Method()),
			attribute.String(string(semconv.HTTPClientIPKey), c.IP()),
			attribute.Int(string(semconv.HTTPStatusCodeKey), c.Response().StatusCode()),
		)
		return err
	}
}
