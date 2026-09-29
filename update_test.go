package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewerCommitAvailable verifies that only a newer published commit raises the warning.
func TestNewerCommitAvailable(t *testing.T) {
	revision := strings.Repeat("a", 40)
	testCases := []struct {
		name         string
		statusCode   int
		responseBody string
		want         bool
	}{
		{name: "newer commit", statusCode: http.StatusOK, responseBody: `{"status":"ahead"}`, want: true},
		{name: "current commit", statusCode: http.StatusOK, responseBody: `{"status":"identical"}`},
		{name: "local commit ahead", statusCode: http.StatusOK, responseBody: `{"status":"behind"}`},
		{name: "unavailable", statusCode: http.StatusServiceUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/compare/"+revision+"...HEAD" || request.URL.Query().Get("per_page") != "1" {
					t.Errorf("unexpected comparison request: %s", request.URL.String())
				}
				if request.Header.Get("User-Agent") != "lgit" {
					t.Error("comparison request is missing its user agent")
				}
				writer.WriteHeader(testCase.statusCode)
				fmt.Fprint(writer, testCase.responseBody)
			}))
			defer server.Close()

			got := newerCommitAvailable(server.Client(), server.URL+"/compare/", revision)
			if got != testCase.want {
				t.Errorf("newerCommitAvailable() = %t, want %t", got, testCase.want)
			}
		})
	}
}
