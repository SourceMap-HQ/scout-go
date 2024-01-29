package echo

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/scout-inc/scout-go"
	"github.com/scout-inc/scout-go/middleware"
	"go.opentelemetry.io/otel/attribute"
)

// echo-compatible middlware
func Middleware() echo.MiddlewareFunc {
	middleware.AssertScoutIsRunning()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			requestDetails := c.Request().Header.Get(scout.RequestTracerHeader)
			sessionSecureId, requestId, err := scout.ExtractIdsFromRequest(requestDetails)
			if err == nil {
				ctx = context.WithValue(ctx, scout.ContextKeys.SessionSecureID, sessionSecureId)
				ctx = context.WithValue(ctx, scout.ContextKeys.RequestID, requestId)
			}

			span, scoutContext := scout.StartTrace(ctx, scout.ScopedKey("echo", nil))
			defer scout.EndTrace(span)

			c.SetRequest(c.Request().WithContext(scoutContext))
			err = next(c)

			span.SetAttributes(attribute.String(scout.SourceAttribute, "GoEchoMiddleware"))
			span.SetAttributes(middleware.GetRequestAttributes(c.Request())...)

			if err != nil {
				scout.RecordSpanError(span, err)
			}

			return err
		}
	}
}
