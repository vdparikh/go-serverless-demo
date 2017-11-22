package main

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
)

// LoggingMiddleware ...
// This is a basic middleware called for all API request and captures some basic logging
// Middleware also verifies if the request is legit and is coming from API GW via Headers
func LoggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Read the API Context
		var ctx interface{}
		_ = json.Unmarshal([]byte(r.Header.Get("X-ApiGatewayProxy-Context")), &ctx)
		requestContextMap := ctx.(map[string]interface{})
		requestID := requestContextMap["aws_request_id"].(string)

		// Create a map for all fields we like to log
		var logFields = make(map[string]interface{})

		// Capture all fields from Context
		for k, v := range requestContextMap {
			if k != "" && v != nil {
				logFields[k] = v
			}
		}

		// Capture few other headers
		logFields["request-id"] = requestID
		logFields["x-forwarded-for"] = r.Header.Get("x-forwarded-for")
		logFields["user-agent"] = r.Header.Get("user-agent")

		// If the X-ApiGatewayProxy-Event header is not there
		// then the request is not coming from the gateway and so reject it
		var e interface{}
		err := json.Unmarshal([]byte(r.Header.Get("X-ApiGatewayProxy-Event")), &e)
		if err != nil {
			logFields["error"] = "Non APIGW Request"
			log.WithFields(logFields).Error("Request")
			respondWithJSON(w, http.StatusUnauthorized, nil)
			return
		}

		// Capture all fields from Event
		eventMap := e.(map[string]interface{})
		for k, v := range eventMap {
			if k != "" && v != nil {
				logFields[k] = v
			}
		}

		// Forward request id downstream so that logging can be simplified
		r.Header.Set("aws-request-id", requestID)

		// Roundtrip
		h.ServeHTTP(w, r)

		// Capture duration
		logFields["duration"] = time.Since(start)

		// Log everything
		log.WithFields(logFields).Info("Request")
	})
}
