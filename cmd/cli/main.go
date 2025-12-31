// Package main provides the CLI for the observability analysis framework
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

var (
	serverURL  = flag.String("server", "http://localhost:8080", "Server URL")
	configPath = flag.String("config", "", "Path to configuration file")
	output     = flag.String("output", "text", "Output format: text, json")
)

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		printUsage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	ctx := context.Background()

	cmd := flag.Args()[0]
	args := flag.Args()[1:]

	var err error
	switch cmd {
	case "analyze":
		err = cmdAnalyze(ctx, client, args)
	case "incident":
		err = cmdIncident(ctx, client, args)
	case "monitor":
		err = cmdMonitor(ctx, client, args)
	case "query":
		err = cmdQuery(ctx, client, args)
	case "status":
		err = cmdStatus(ctx, client, args)
	case "knowledge":
		err = cmdKnowledge(ctx, client, args)
	case "health":
		err = cmdHealth(ctx, client)
	case "init":
		err = cmdInit(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Observability Analysis CLI

Usage: rag-cli [flags] <command> [args]

Commands:
  analyze   Run observability analysis
  incident  Analyze an incident
  monitor   Start/stop continuous monitoring
  query     Natural language query
  status    Get workflow status
  knowledge Manage knowledge base
  health    Check server health
  init      Initialize configuration

Flags:`)
	flag.PrintDefaults()
	fmt.Println(`
Examples:
  rag-cli analyze "Why is checkout slow?"
  rag-cli analyze -services=api,db -time=60m "High error rate investigation"
  rag-cli incident -title="API Outage" -desc="Users reporting 500 errors"
  rag-cli monitor start -services=api,db -interval=5m
  rag-cli query "What are common causes of high latency?"
  rag-cli knowledge add -type=runbook -title="High CPU" -file=runbook.md
  rag-cli health`)
}

func cmdAnalyze(ctx context.Context, client *http.Client, args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	services := fs.String("services", "", "Comma-separated list of services")
	timeRange := fs.String("time", "60m", "Time range (e.g., 30m, 1h, 24h)")
	analysisType := fs.String("type", "all", "Analysis type: trace, metric, log, all")
	includeRAG := fs.Bool("rag", true, "Include knowledge base context")
	async := fs.Bool("async", false, "Run asynchronously")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("query required")
	}
	query := strings.Join(fs.Args(), " ")

	// Parse time range
	duration, err := time.ParseDuration(*timeRange)
	if err != nil {
		return fmt.Errorf("invalid time range: %v", err)
	}

	var svcList []string
	if *services != "" {
		svcList = strings.Split(*services, ",")
	}

	reqBody := map[string]interface{}{
		"query":              query,
		"services":           svcList,
		"time_range_minutes": int(duration.Minutes()),
		"analysis_type":      *analysisType,
		"include_rag":        *includeRAG,
		"async":              *async,
	}

	body, _ := json.Marshal(reqBody)
	resp, err := client.Post(*serverURL+"/api/v1/analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func cmdIncident(ctx context.Context, client *http.Client, args []string) error {
	fs := flag.NewFlagSet("incident", flag.ExitOnError)
	title := fs.String("title", "", "Incident title")
	desc := fs.String("desc", "", "Incident description")
	services := fs.String("services", "", "Comma-separated list of affected services")
	startTime := fs.String("start", "", "Incident start time (RFC3339)")
	symptoms := fs.String("symptoms", "", "Comma-separated list of symptoms")
	fs.Parse(args)

	if *title == "" || *desc == "" {
		return fmt.Errorf("title and desc required")
	}

	var svcList []string
	if *services != "" {
		svcList = strings.Split(*services, ",")
	}

	var symptomList []string
	if *symptoms != "" {
		symptomList = strings.Split(*symptoms, ",")
	}

	reqBody := map[string]interface{}{
		"title":       *title,
		"description": *desc,
		"services":    svcList,
		"start_time":  *startTime,
		"symptoms":    symptomList,
	}

	body, _ := json.Marshal(reqBody)
	resp, err := client.Post(*serverURL+"/api/v1/analyze/incident", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func cmdMonitor(ctx context.Context, client *http.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("subcommand required: start, stop")
	}

	subcmd := args[0]
	args = args[1:]

	switch subcmd {
	case "start":
		fs := flag.NewFlagSet("monitor start", flag.ExitOnError)
		services := fs.String("services", "", "Comma-separated list of services")
		interval := fs.String("interval", "5m", "Check interval")
		lookback := fs.String("lookback", "15m", "Lookback window")
		fs.Parse(args)

		intervalDur, _ := time.ParseDuration(*interval)
		lookbackDur, _ := time.ParseDuration(*lookback)

		var svcList []string
		if *services != "" {
			svcList = strings.Split(*services, ",")
		}

		reqBody := map[string]interface{}{
			"services":         svcList,
			"interval_minutes": int(intervalDur.Minutes()),
			"lookback_minutes": int(lookbackDur.Minutes()),
		}

		body, _ := json.Marshal(reqBody)
		resp, err := client.Post(*serverURL+"/api/v1/monitor/start", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return handleResponse(resp)

	case "stop":
		fs := flag.NewFlagSet("monitor stop", flag.ExitOnError)
		workflowID := fs.String("id", "", "Workflow ID to stop")
		fs.Parse(args)

		if *workflowID == "" {
			return fmt.Errorf("workflow ID required")
		}

		url := fmt.Sprintf("%s/api/v1/monitor/stop?workflow_id=%s", *serverURL, *workflowID)
		resp, err := client.Post(url, "application/json", nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return handleResponse(resp)

	default:
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}
}

func cmdQuery(ctx context.Context, client *http.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("query required")
	}

	query := strings.Join(args, " ")

	reqBody := map[string]interface{}{
		"query": query,
	}

	body, _ := json.Marshal(reqBody)
	resp, err := client.Post(*serverURL+"/api/v1/query", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func cmdStatus(ctx context.Context, client *http.Client, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	workflowID := fs.String("id", "", "Workflow ID")
	fs.Parse(args)

	if *workflowID == "" {
		return fmt.Errorf("workflow ID required")
	}

	url := fmt.Sprintf("%s/api/v1/analyze/status?workflow_id=%s", *serverURL, *workflowID)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func cmdKnowledge(ctx context.Context, client *http.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("subcommand required: add, query")
	}

	subcmd := args[0]
	args = args[1:]

	switch subcmd {
	case "add":
		fs := flag.NewFlagSet("knowledge add", flag.ExitOnError)
		docType := fs.String("type", "general", "Document type: runbook, incident, architecture, pattern")
		title := fs.String("title", "", "Document title")
		content := fs.String("content", "", "Document content")
		file := fs.String("file", "", "Read content from file")
		tags := fs.String("tags", "", "Comma-separated tags")
		fs.Parse(args)

		if *title == "" {
			return fmt.Errorf("title required")
		}

		contentStr := *content
		if *file != "" {
			data, err := os.ReadFile(*file)
			if err != nil {
				return err
			}
			contentStr = string(data)
		}

		if contentStr == "" {
			return fmt.Errorf("content required")
		}

		var tagList []string
		if *tags != "" {
			tagList = strings.Split(*tags, ",")
		}

		reqBody := map[string]interface{}{
			"type":    *docType,
			"title":   *title,
			"content": contentStr,
			"tags":    tagList,
		}

		body, _ := json.Marshal(reqBody)
		resp, err := client.Post(*serverURL+"/api/v1/knowledge/add", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return handleResponse(resp)

	case "query":
		fs := flag.NewFlagSet("knowledge query", flag.ExitOnError)
		topK := fs.Int("k", 5, "Number of results")
		docTypes := fs.String("types", "", "Document types to filter")
		fs.Parse(args)

		if fs.NArg() < 1 {
			return fmt.Errorf("query required")
		}

		query := strings.Join(fs.Args(), " ")

		var types []string
		if *docTypes != "" {
			types = strings.Split(*docTypes, ",")
		}

		reqBody := map[string]interface{}{
			"query": query,
			"top_k": *topK,
			"types": types,
		}

		body, _ := json.Marshal(reqBody)
		resp, err := client.Post(*serverURL+"/api/v1/knowledge/query", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return handleResponse(resp)

	default:
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}
}

func cmdHealth(ctx context.Context, client *http.Client) error {
	resp, err := client.Get(*serverURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleResponse(resp)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	outputPath := fs.String("o", "config.yaml", "Output path")
	fs.Parse(args)

	cfg := config.DefaultConfig()
	return cfg.Save(*outputPath)
}

func handleResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	if *output == "json" {
		fmt.Println(string(body))
		return nil
	}

	// Pretty print for text output
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	return printResult(result, 0)
}

func printResult(data interface{}, indent int) error {
	prefix := strings.Repeat("  ", indent)

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			switch val := value.(type) {
			case map[string]interface{}, []interface{}:
				fmt.Printf("%s%s:\n", prefix, key)
				printResult(val, indent+1)
			default:
				fmt.Printf("%s%s: %v\n", prefix, key, value)
			}
		}
	case []interface{}:
		for i, item := range v {
			switch val := item.(type) {
			case map[string]interface{}:
				fmt.Printf("%s[%d]:\n", prefix, i)
				printResult(val, indent+1)
			default:
				fmt.Printf("%s- %v\n", prefix, item)
			}
		}
	default:
		fmt.Printf("%s%v\n", prefix, v)
	}

	return nil
}

// Ensure types is used
var _ types.AnalysisRequest
