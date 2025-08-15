// MCP Server for AI Colleague Integration
// This server provides AI analysis tools via Model Context Protocol
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/minz/minzc/internal/mcp"
)

func main() {
	// Check if running as MCP server
	if len(os.Args) > 1 && os.Args[1] == "mcp-server" {
		runMCPServer()
		return
	}

	// Show usage
	fmt.Println("MinZ AI Colleague MCP Server")
	fmt.Println("Usage:")
	fmt.Println("  mcp-ai-colleague mcp-server    # Run as MCP server")
	fmt.Println("")
	fmt.Println("Environment variables:")
	fmt.Println("  AZURE_OPENAI_ENDPOINT - Azure OpenAI endpoint")
	fmt.Println("  AZURE_OPENAI_API_KEY  - Azure OpenAI API key")
	fmt.Println("  AZURE_OPENAI_DEPLOYMENT - Deployment name (default: gpt-4-1106)")
}

func runMCPServer() {
	// Create handler
	handler := mcp.NewAIHandler()

	// Create server
	server := mcp.NewServer("minz-ai", "0.1.0", handler)

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Start server
	if err := server.Start(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}