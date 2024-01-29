package scout

import (
	"errors"
	"strings"
)

const (
	ScoutInternalLogTag = "[scout-go]"
	RequestTracerHeader = "X-Scout-Request"
)

func ExtractIdsFromRequest(requestDetails string) (string, string, error) {
	sessionSecureId := ""
	requestId := ""

	ids := strings.Split(requestDetails, "/")
	if len(ids) >= 2 {
		sessionSecureId = ids[0]
		requestId = ids[1]

		return sessionSecureId, requestId, nil
	}

	return sessionSecureId, requestId, errors.New("request does not contain tracer IDs")
}
