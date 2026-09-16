// Command estus-mcp connects AI apps that only speak MCP over stdio (Claude
// Desktop, for one) to an Estus Brain server. It is a thin relay: it lists the
// server's tools over streamable HTTP and forwards every call, so it needs no
// database — just the server's URL, its token and, behind the mTLS edge, a
// client certificate.
//
// Claude Desktop config (claude_desktop_config.json):
//
//	"mcpServers": {
//	  "estus-brain": {
//	    "command": "/path/to/estus-mcp",
//	    "env": {
//	      "ESTUS_MCP_URL": "https://brain.example.com/mcp",
//	      "ESTUS_MCP_TOKEN": "estus_…",
//	      "ESTUS_CLIENT_CERT": "/path/client.crt",
//	      "ESTUS_CLIENT_KEY": "/path/client.key"
//	    }
//	  }
//	}
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type authTransport struct {
	token string
	base  http.RoundTripper
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

func main() {
	// stdout belongs to the protocol; logs go to stderr.
	log.SetOutput(os.Stderr)
	if err := run(); err != nil {
		log.Fatalf("estus-mcp: %v", err)
	}
}

func run() error {
	url := os.Getenv("ESTUS_MCP_URL")
	if url == "" {
		url = "http://127.0.0.1:8080/mcp"
	}
	token := os.Getenv("ESTUS_MCP_TOKEN")
	if token == "" {
		return fmt.Errorf("ESTUS_MCP_TOKEN is required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cert, key := os.Getenv("ESTUS_CLIENT_CERT"), os.Getenv("ESTUS_CLIENT_KEY"); cert != "" && key != "" {
		pair, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return fmt.Errorf("client certificate: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "estus-mcp-relay", Version: "1.0.0"}, nil)
	remote, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: &http.Client{Transport: authTransport{token: token, base: transport}},
	}, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", url, err)
	}
	defer remote.Close()

	init := remote.InitializeResult()
	instructions := ""
	if init != nil {
		instructions = init.Instructions
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "estus-brain", Title: "Estus Brain", Version: "1.0.0"}, &mcp.ServerOptions{Instructions: instructions})

	for tool, err := range remote.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("list tools: %w", err)
		}
		name := tool.Name
		server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return remote.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: req.Params.Arguments})
		})
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
