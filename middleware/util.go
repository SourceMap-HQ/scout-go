package middleware

import (
	"net/http"
	"strings"

	"github.com/scout-inc/scout-go"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func AssertScoutIsRunning() {
	if !scout.IsRunning() {
		logrus.Errorf("%s middleware added but Scout is not running. did you forget to run `Scout.Init(); defer H.Stop()`?", scout.ScoutInternalLogTag)
	}
}

func GetIPAddress(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Client-IP")
	}
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
		if IPAddress != "" && strings.Contains(IPAddress, ",") {
			if ipList := strings.Split(IPAddress, ","); len(ipList) > 0 {
				IPAddress = ipList[0]
			}
		}
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

func GetRequestAttributes(r *http.Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(string(semconv.HTTPMethodKey), r.Method),
		attribute.String(string(semconv.HTTPClientIPKey), GetIPAddress(r)),
	}
	if r.URL != nil {
		attrs = append(attrs,
			attribute.String(string(semconv.HTTPURLKey), r.URL.String()),
			attribute.String(string(semconv.HTTPRouteKey), r.URL.RequestURI()),
		)
	}
	if r.Response != nil {
		attrs = append(attrs, attribute.Int(string(semconv.HTTPStatusCodeKey), r.Response.StatusCode))
	}
	return attrs
}
