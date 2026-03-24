package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/orchestrator"
)

func main() {
	// Top-level panic recovery so we always return valid JSON to the Java caller.
	defer func() {
		if r := recover(); r != nil {
			errResult := models.AnalysisResult{
				Status:       "error",
				ErrorMessage: fmt.Sprintf("fatal panic: %v", r),
			}
			out, _ := json.Marshal(errResult)
			fmt.Fprintln(os.Stderr, string(out))
			os.Exit(1)
		}
	}()

	// Define flags.
	analyzeCmd := flag.NewFlagSet("analyze", flag.ExitOnError)
	requestJSON := analyzeCmd.String("request", "", "JSON-encoded AnalysisRequest")
	async := analyzeCmd.Bool("async", false, "Run analysis asynchronously; print analysis ID and continue in background")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mcp-agent analyze --request='{...}' [--async]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "analyze":
		analyzeCmd.Parse(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(1)
	}

	if *requestJSON == "" {
		fmt.Fprintln(os.Stderr, "error: --request is required")
		os.Exit(1)
	}

	// Parse the incoming request.
	var req models.AnalysisRequest
	if err := json.Unmarshal([]byte(*requestJSON), &req); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing request JSON: %v\n", err)
		os.Exit(1)
	}

	// Assign an analysis ID if one was not provided.
	if req.AnalysisID == "" {
		req.AnalysisID = uuid.New().String()
	}

	ctx := context.Background()

	if *async {
		// Async mode: print the analysis ID immediately so the Java process can
		// read it from stdout and return to the user. The Go process continues
		// running in the foreground until the analysis completes.
		fmt.Printf("Analysis submitted. ID: %s\n", req.AnalysisID)

		result, err := orchestrator.Analyze(ctx, &req)
		if err != nil {
			log.Printf("async analysis %s failed: %v", req.AnalysisID, err)
			os.Exit(1)
		}
		log.Printf("async analysis %s completed: status=%s", req.AnalysisID, result.Status)
	} else {
		// Synchronous mode: run analysis and print the result JSON to stdout.
		result, err := orchestrator.Analyze(ctx, &req)
		if err != nil {
			errResult := models.AnalysisResult{
				Status:       "error",
				ErrorMessage: err.Error(),
			}
			out, _ := json.Marshal(errResult)
			fmt.Println(string(out))
			os.Exit(1)
		}

		out, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error marshalling result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	}
}
