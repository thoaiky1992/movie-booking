package http

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type healthCheckerStub struct {
	status map[string]string
}

func (s *healthCheckerStub) Health() map[string]string {
	return s.status
}

func TestSystemHandlerRootUsesSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewSystemHandler(&healthCheckerStub{})
	router := gin.New()
	router.GET("/", handler.Root)

	request := httptest.NewRequest(nethttp.MethodGet, "/", nil)
	request.Header.Set(TraceIDHeader, "trace-success-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, nethttp.StatusOK)
	}
	if got := recorder.Header().Get(TraceIDHeader); got != "trace-success-123" {
		t.Fatalf("trace header = %q, want %q", got, "trace-success-123")
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Data.Message != "Hello World" || response.TraceID != "trace-success-123" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSystemHandlerHealthReturnsCheckerStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewSystemHandler(&healthCheckerStub{status: map[string]string{"status": "up"}})
	router := gin.New()
	router.GET("/health", handler.Health)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, nethttp.StatusOK)
	}
	var response struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data["status"] != "up" {
		t.Fatalf("health status = %q, want up", response.Data["status"])
	}
}
