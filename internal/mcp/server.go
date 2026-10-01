// Package mcp exposes Ninebot APIs as MCP tools (`ninecli mcp`).
package mcp

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ninecli/internal/api"
	"ninecli/internal/config"
)

// Runtime bundles everything tools need.
type Runtime struct {
	Dir    config.Dir
	Hosts  api.Hosts
	client *api.Client
}

// Client lazily builds the API client from tokens on disk.
func (rt *Runtime) Client() (*api.Client, *config.Tokens, error) {
	if rt.client != nil && rt.client.Tokens != nil {
		return rt.client, rt.client.Tokens, nil
	}
	cfg, err := rt.Dir.Load()
	if err != nil {
		return nil, nil, err
	}
	tok, err := rt.Dir.LoadTokens()
	if err != nil {
		return nil, nil, err
	}
	c := api.New(rt.Hosts, cfg, tok)
	rt.client = c
	return c, tok, nil
}

// Invalidate drops the cached client so fresh tokens get picked up.
func (rt *Runtime) Invalidate() { rt.client = nil }

func okResult(data string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: data}}}
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

// NewServer registers all tools on an MCP server.
func NewServer(rt *Runtime) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "ninecli", Version: "0.1.7"}, nil)
	registerTools(server, rt)
	return server
}

// RunStdio serves MCP over stdio until the client disconnects.
func RunStdio(ctx context.Context, rt *Runtime) error {
	server := NewServer(rt)
	err := server.Run(ctx, &mcp.StdioTransport{})
	if err != nil {
		msg := err.Error()
		// Client closing stdin is a normal shutdown, not a failure.
		if msg == "EOF" || strings.Contains(msg, "server is closing") {
			return nil
		}
	}
	return err
}

