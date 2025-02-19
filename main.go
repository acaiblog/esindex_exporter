package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
)

var (
	esURI         = flag.String("es-uri", "", "Elasticsearch URI in the format http://username:password@es-ip:9200 (required)")
	esIndexPrefix = flag.String("es-index-prefix", "", "Elasticsearch Index Prefix (required)")
	queryInterval = flag.Int("query-interval", 10, "Query interval in seconds (required)")
	listenPort    = flag.Int("listen-port", 9184, "Port to listen for metrics")
	startTimeStr  = flag.String("start-time", "09:00", "Start time in HH:MM format (default is 09:00)")
	endTimeStr    = flag.String("end-time", "18:00", "End time in HH:MM format (default is 18:00)")
	timeoutStr    = flag.String("timeout", "5s", "Timeout in seconds (default is 5s)")
)

// Define custom Prometheus metrics
var (
	indexExistsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasticsearch_indices_exists",
			Help: "Whether an Elasticsearch index exists (1 if exists, 0 otherwise)",
		},
		[]string{"index_name"},
	)
)

func init() {
	prometheus.MustRegister(indexExistsGauge)
}

func parseTime(timeStr string) (time.Time, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}
	return t, nil
}

func main() {
	flag.Parse()

	if *esURI == "" || *esIndexPrefix == "" || *queryInterval == 0 {
		fmt.Println("Error: All parameters are required.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	startTime, err := parseTime(*startTimeStr)
	if err != nil {
		log.Fatalf("Error parsing start time: %v", err)
	}
	endTime, err := parseTime(*endTimeStr)
	if err != nil {
		log.Fatalf("Error parsing end time: %v", err)
	}

	parsedURI, err := url.Parse(*esURI)
	log.Println("parsedURI:", parsedURI)
	if err != nil {
		log.Fatalf("Error parsing Elasticsearch URI: %s", err)
	}

	var username, password string
	if parsedURI.User != nil {
		username = parsedURI.User.Username()
		password, _ = parsedURI.User.Password() // 如果没有密码，password 将是空字符串
	}

	var timeout time.Duration
	timeout, err = time.ParseDuration(*timeoutStr)
	if err != nil {
		log.Fatalf("Error parsing timeout: %s", err)
	}

	// 自定义HTTP传输以设置连接超时
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout, // 设置连接超时时间
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if transport == nil {
		log.Fatal("Failed to create HTTP transport")
	}

	// 配置Elasticsearch客户端
	cfg := elasticsearch.Config{
		Addresses: []string{parsedURI.Scheme + "://" + parsedURI.Host},
		Username:  username,
		Password:  password,
		Transport: transport,
	}
	log.Println("host:", parsedURI.Host, "username:", username, "password:", password)
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	// 进行健康检查
	res, err := client.Ping()
	if err != nil {
		log.Fatalf("Error pinging Elasticsearch cluster: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Fatalf("Elasticsearch cluster returned an error: %s", res.String())
	}
	log.Printf("Connected to Elasticsearch cluster, version: %s", res.String())
	printAllIndexes(client)

	// Start HTTP server for metrics
	go startMetricsServer(*listenPort)

	ticker := time.NewTicker(time.Duration(*queryInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentTime := time.Now()
			currentTimeString := currentTime.Format("15:04")

			currentParsedTime, err := parseTime(currentTimeString)
			if err != nil {
				log.Printf("Error parsing current time: %v", err)
				continue
			}

			today := currentTime.Format("2006.01.02")
			indexName := *esIndexPrefix + today

			// 检查索引是否存在
			indexExists, err := checkIndexExists(client, indexName)
			if err != nil {
				log.Printf("Error checking index: %s", err)
				continue
			}

			if currentParsedTime.Before(startTime) || currentParsedTime.After(endTime) {
				// 当前时间不在指定时间段内，设置指标为1
				indexExistsGauge.WithLabelValues(indexName).Set(1)
			} else {
				// 当前时间在指定时间段内
				if indexExists {
					// 索引存在，设置指标为1
					indexExistsGauge.WithLabelValues(indexName).Set(1)
				} else {
					// 索引不存在，设置指标为0
					indexExistsGauge.WithLabelValues(indexName).Set(0)
				}
			}
		}
	}
}

func checkIndexExists(client *elasticsearch.Client, indexName string) (bool, error) {
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	resp, err := req.Do(context.Background(), client)
	if err != nil {
		return false, fmt.Errorf("failed to query index existence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	}

	return true, nil
}

func printAllIndexes(client *elasticsearch.Client) {
	req := esapi.IndicesGetRequest{
		Index: []string{"*"},
	}

	resp, err := req.Do(context.Background(), client)
	if err != nil {
		log.Printf("Error fetching indexes: %s", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Error decoding response body: %s", err)
		return
	}

	log.Printf("Found %d indexes", len(result))
	for indexName := range result {
		log.Printf("Index name: %s", indexName)
	}
}

func startMetricsServer(port int) {
	// Serve the root path with a simple HTML page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `
<html>
<head><title>ES Index Exporter</title></head>
<body>
<h1>ES Index Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
</body>
</html>
`)
	})

	// Serve the /metrics endpoint using Prometheus handler
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting metrics server on port %d", port)
	log.Printf("Metrics server Running Range: %s - %s", *startTimeStr, *endTimeStr)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Failed to start metrics server: %s", err)
	}
}
