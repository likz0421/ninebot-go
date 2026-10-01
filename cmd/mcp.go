package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	ninebotmcp "ninecli/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run a Model Context Protocol server for Ninebot APIs",
	Long: "Run an MCP server exposing Ninebot auth, vehicle, travel, and control tools.\n\n" +
		"Default mode uses stdio for local MCP clients. Use --http to run a Streamable\n" +
		"HTTP MCP server. HTTP bearer auth is optional and controlled by --token or\n" +
		"NINEBOT_SERVE_TOKEN.",
	RunE: func(c *cobra.Command, args []string) error {
		return runMCP(c, args)
	},
}

var mcpFlags struct {
	http  bool
	bind  string
	token string
	quiet bool
}

func init() {
	mcpCmd.Flags().BoolVar(&mcpFlags.http, "http", false, "run Streamable HTTP instead of stdio")
	mcpCmd.Flags().StringVar(&mcpFlags.bind, "bind", "", "HTTP address to listen on (host:port)")
	mcpCmd.Flags().StringVar(&mcpFlags.token, "token", "", "require Authorization: Bearer <token> for HTTP mode")
	mcpCmd.Flags().BoolVar(&mcpFlags.quiet, "quiet", false, "suppress non-error HTTP request logs")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(c *cobra.Command, args []string) error {
	rt := &ninebotmcp.Runtime{Dir: dirOrDefaults(), Hosts: hostsOrDefaults()}
	if !mcpFlags.http {
		return ninebotmcp.RunStdio(c.Context(), rt)
	}
	bind := resolveMCPBind()
	token := resolveMCPToken()
	server := ninebotmcp.NewServer(rt)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)
	if !mcpFlags.quiet {
		log.Printf("ninecli mcp (streamable http) listening on %s", bind)
		if token == "" && !ninebotmcp.LoopbackOnly(bind) {
			log.Printf("warning: binding non-loopback without --token")
		}
	}
	return http.ListenAndServe(bind, ninebotmcp.RequireBearer(token, handler))
}

func resolveMCPBind() string {
	if mcpFlags.bind != "" {
		return mcpFlags.bind
	}
	if v := os.Getenv("NINEBOT_SERVE_BIND"); v != "" {
		return v
	}
	return "127.0.0.1:18019"
}

func resolveMCPToken() string {
	if mcpFlags.token != "" {
		return mcpFlags.token
	}
	return os.Getenv("NINEBOT_SERVE_TOKEN")
}

var _ = fmt.Sprintf
