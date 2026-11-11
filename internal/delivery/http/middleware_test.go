package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryUsesErrorEnvelopeAndTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	r := gin.New()
	r.Use(TraceIDMiddleware(), RequestLogger(logger), Recovery(logger))
	r.GET("/panic", func(_ *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest(nethttp.MethodGet, "/panic", nil)
	req.Header.Set(TraceIDHeader, "trace-panic-789")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, nethttp.StatusInternalServerError)
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Success || response.Error.Code != "INTERNAL_ERROR" || response.TraceID != "trace-panic-789" {
		t.Fatalf("unexpected response: %+v", response)
	}

	decoder := json.NewDecoder(&logOutput)
	var panicEntry map[string]any
	if err := decoder.Decode(&panicEntry); err != nil {
		t.Fatalf("decode panic log entry: %v", err)
	}
	if panicEntry["msg"] != "http_panic_recovered" || panicEntry["trace_id"] != "trace-panic-789" {
		t.Fatalf("unexpected panic log entry: %+v", panicEntry)
	}

	var requestEntry map[string]any
	if err := decoder.Decode(&requestEntry); err != nil {
		t.Fatalf("decode request log entry: %v", err)
	}
	if requestEntry["msg"] != "http_request" || requestEntry["trace_id"] != "trace-panic-789" {
		t.Fatalf("unexpected request log entry: %+v", requestEntry)
	}
}

func TestTraceIDMiddlewareReplacesInvalidIncomingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(TraceIDMiddleware())
	r.GET("/", func(c *gin.Context) {
		RespondSuccess(c, nethttp.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	req.Header.Set(TraceIDHeader, "invalid trace id")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	traceID := recorder.Header().Get(TraceIDHeader)
	if len(traceID) != 32 || traceID == "invalid trace id" {
		t.Fatalf("generated trace ID = %q, want 32-character ID", traceID)
	}
}
