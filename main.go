// nzbget-exporter exposes NZBGet statistics as Prometheus metrics. It queries
// NZBGet's JSON-RPC API (status, servervolumes, config) on every scrape.
//
// Configuration (environment):
//
//	NZBGET_HOST      base URL of the NZBGet instance, e.g. http://nzbget:6789 (required)
//	NZBGET_USERNAME  basic-auth username (optional)
//	NZBGET_PASSWORD  basic-auth password (optional)
//	NZBGET_LISTEN    address to listen on (default :9452)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "nzbget"

type rpcClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func (c *rpcClient) call(method string, result any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/jsonrpc/"+method, nil)
	if err != nil {
		return err
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected status %s", method, resp.Status)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if envelope.Error != nil {
		return fmt.Errorf("%s: RPC error: %s", method, envelope.Error.Message)
	}
	return json.Unmarshal(envelope.Result, result)
}

type statusResult struct {
	ThreadCount float64 `json:"ThreadCount"`
}

type serverVolume struct {
	ServerID    int     `json:"ServerID"`
	TotalSizeLo float64 `json:"TotalSizeLo"`
	TotalSizeHi float64 `json:"TotalSizeHi"`
}

// totalBytes recombines NZBGet's split 64-bit counter.
func (v serverVolume) totalBytes() float64 {
	return v.TotalSizeHi*4294967296 + v.TotalSizeLo
}

type configEntry struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type collector struct {
	rpc *rpcClient

	up          *prometheus.Desc
	threadCount *prometheus.Desc
	serverBytes *prometheus.Desc
}

func newCollector(rpc *rpcClient) *collector {
	return &collector{
		rpc: rpc,
		up: prometheus.NewDesc(namespace+"_up",
			"Whether the last NZBGet API query succeeded (1) or failed (0).", nil, nil),
		threadCount: prometheus.NewDesc(namespace+"_thread_count",
			"Number of threads in the NZBGet process.", nil, nil),
		serverBytes: prometheus.NewDesc(namespace+"_news_server_total_bytes",
			"Total bytes downloaded from a news server since counters were reset.",
			[]string{"server"}, nil),
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.threadCount
	ch <- c.serverBytes
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	var status statusResult
	if err := c.rpc.call("status", &status); err != nil {
		log.Printf("nzbget API query failed: %v", err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(c.threadCount, prometheus.GaugeValue, status.ThreadCount)

	var volumes []serverVolume
	if err := c.rpc.call("servervolumes", &volumes); err != nil {
		log.Printf("servervolumes query failed: %v", err)
		return
	}
	names := c.serverNames()
	for _, v := range volumes {
		// Entry 0 is the all-servers aggregate; per-server entries start at 1.
		// Skipping it keeps sum() over the series equal to the real total.
		if v.ServerID == 0 {
			continue
		}
		name := names[v.ServerID]
		if name == "" {
			name = fmt.Sprintf("server%d", v.ServerID)
		}
		ch <- prometheus.MustNewConstMetric(c.serverBytes, prometheus.CounterValue,
			v.totalBytes(), name)
	}
}

// serverNames maps server IDs to their configured Server<N>.Name (falling back
// to Server<N>.Host). Best-effort: on error every server falls back to
// server<N>.
func (c *collector) serverNames() map[int]string {
	var cfg []configEntry
	if err := c.rpc.call("config", &cfg); err != nil {
		log.Printf("config query failed: %v", err)
		return nil
	}
	names := map[int]string{}
	hosts := map[int]string{}
	for _, e := range cfg {
		// Entry names look like Server1.Name / Server1.Host. Sscanf cannot be
		// used here: it counts the %d as matched before the literal suffix
		// fails, so .Host entries would also match a Server%d.Name format.
		key, ok := strings.CutPrefix(e.Name, "Server")
		if !ok {
			continue
		}
		idStr, field, ok := strings.Cut(key, ".")
		if !ok {
			continue
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		switch field {
		case "Name":
			names[id] = e.Value
		case "Host":
			hosts[id] = e.Value
		}
	}
	for id, host := range hosts {
		if names[id] == "" {
			names[id] = host
		}
	}
	return names
}

func main() {
	host := os.Getenv("NZBGET_HOST")
	if host == "" {
		log.Fatal("NZBGET_HOST is required")
	}
	listen := os.Getenv("NZBGET_LISTEN")
	if listen == "" {
		listen = ":9452"
	}

	rpc := &rpcClient{
		baseURL:  strings.TrimRight(host, "/"),
		username: os.Getenv("NZBGET_USERNAME"),
		password: os.Getenv("NZBGET_PASSWORD"),
		client:   &http.Client{Timeout: 10 * time.Second},
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(newCollector(rpc))

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Printf("listening on %s", listen)
	log.Fatal(http.ListenAndServe(listen, nil))
}
