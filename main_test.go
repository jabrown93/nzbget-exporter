package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func rpcFixtureServer(t *testing.T, responses map[string][]byte, username, password string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if username != "" || password != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != username || pass != password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		method := strings.TrimPrefix(r.URL.Path, "/jsonrpc/")
		body, ok := responses[method]
		if !ok || r.URL.Path != "/jsonrpc/"+method {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func newTestClient(url string) *rpcClient {
	return &rpcClient{baseURL: url, client: &http.Client{}}
}

func TestRPCClientCallParsesStatus(t *testing.T) {
	server := rpcFixtureServer(t, map[string][]byte{
		"status": fixture(t, "status.json"),
	}, "exporter", "secret")
	defer server.Close()

	client := newTestClient(server.URL)
	client.username = "exporter"
	client.password = "secret"
	var status statusResult
	if err := client.call("status", &status); err != nil {
		t.Fatal(err)
	}
	if status.ThreadCount != 42 {
		t.Errorf("ThreadCount = %v, want 42", status.ThreadCount)
	}
}

func TestCollectorCollectsServerMetrics(t *testing.T) {
	server := rpcFixtureServer(t, map[string][]byte{
		"status":        fixture(t, "status.json"),
		"servervolumes": fixture(t, "servervolumes.json"),
		"config":        fixture(t, "config.json"),
	}, "", "")
	defer server.Close()

	collector := newCollector(newTestClient(server.URL))
	if count := testutil.CollectAndCount(collector); count != 4 {
		t.Errorf("metric count = %d, want 4", count)
	}
	if err := testutil.CollectAndCompare(collector, strings.NewReader(`
# HELP nzbget_news_server_total_bytes Total bytes downloaded from a news server since counters were reset.
# TYPE nzbget_news_server_total_bytes counter
nzbget_news_server_total_bytes{server="news.backup.example"} 2048
nzbget_news_server_total_bytes{server="primary"} 4.29496832e+09
# HELP nzbget_thread_count Number of threads in the NZBGet process.
# TYPE nzbget_thread_count gauge
nzbget_thread_count 42
# HELP nzbget_up Whether the last NZBGet API query succeeded (1) or failed (0).
# TYPE nzbget_up gauge
nzbget_up 1
`), "nzbget_up", "nzbget_thread_count", "nzbget_news_server_total_bytes"); err != nil {
		t.Error(err)
	}
}

func TestCollectorReportsDownForStatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		client  func(string) *rpcClient
	}{
		{
			name: "API unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}),
			client: newTestClient,
		},
		{
			name: "invalid JSON",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(fixture(t, "invalid-response.txt"))
			}),
			client: newTestClient,
		},
		{
			name: "authentication failure",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if !ok || user != "exporter" || password != "secret" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = w.Write(fixture(t, "status.json"))
			}),
			client: func(url string) *rpcClient {
				client := newTestClient(url)
				client.username = "exporter"
				client.password = "wrong-password"
				return client
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()

			if err := testutil.CollectAndCompare(newCollector(test.client(server.URL)), strings.NewReader(`
# HELP nzbget_up Whether the last NZBGet API query succeeded (1) or failed (0).
# TYPE nzbget_up gauge
nzbget_up 0
`), "nzbget_up"); err != nil {
				t.Error(err)
			}
		})
	}
}
