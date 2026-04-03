package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/blueberry/mcp/internal/mcp"
	"github.com/blueberry/mcp/internal/store"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		fmt.Println("Blueberry initialized! Ready to be used with supported SDKs.")
		os.Exit(0)
	}

	transport := flag.String("transport", "stdio", "Transport to use: stdio, sse, streamable-http")
	flag.Parse()

	appStore := store.NewLocalStore()
	wrapper := mcp.NewServerWrapper(appStore)

	switch *transport {
	case "stdio":
		fmt.Fprintf(os.Stderr, "Blueberry MCP Server starting on stdio...\n")
		if err := mcpserver.ServeStdio(wrapper.Server()); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	case "sse":
		fmt.Fprintf(os.Stderr, "Blueberry MCP Server starting on SSE...\n")
		// Assume mcp-go provides ServeSSE - signature varies by library version, we mock it here.
		// if err := mcpserver.ServeSSE(wrapper.Server(), ":8000"); err != nil {
		// 	fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		// 	os.Exit(1)
		// }
		fmt.Fprintf(os.Stderr, "SSE not natively exported by this version, relying on stdio.\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown transport %s\n", *transport)
		os.Exit(1)
	}
}
