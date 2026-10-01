package cmd

import (
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"ninecli/internal/api"
	"ninecli/internal/proxy"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run an HTTP proxy exposing Ninebot APIs as plaintext REST",
	Long: "serve starts an HTTP server that translates REST requests into Ninebot's\n" +
		"encrypted Passport + business APIs. Use it to drive ninebot from scripts\n" +
		"or integrations without re-implementing the encryption layer.\n\n" +
		"By default listens on 127.0.0.1:18009 with no authentication. Use --token\n" +
		"to require an Authorization: Bearer header (recommended when binding to\n" +
		"a non-loopback address).\n\n" +
		"Tokens are read from --config/tokens.json on startup (if present) and\n" +
		"written through on every login or refresh. There is no startup\n" +
		"validation; protected endpoints return 401 until POST /auth/login.",
	RunE: func(c *cobra.Command, args []string) error {
		return runServe(c, args)
	},
}

var serveFlags struct {
	bind  string
	token string
	quiet bool
}

func init() {
	serveCmd.Flags().StringVar(&serveFlags.bind, "bind", "127.0.0.1:18009", "address to listen on (host:port)")
	serveCmd.Flags().StringVar(&serveFlags.token, "token", "", "require Authorization: Bearer <token> on every non-/healthz request")
	serveCmd.Flags().BoolVar(&serveFlags.quiet, "quiet", false, "suppress non-error request logs")
	rootCmd.AddCommand(serveCmd)
}

func runServe(c *cobra.Command, args []string) error {
	d := dirOrDefaults()
	cfg, err := d.Load()
	if err != nil {
		return err
	}
	tok, _ := d.LoadTokens()
	bind := serveFlags.bind
	if v := os.Getenv("NINEBOT_SERVE_BIND"); v != "" && serveFlags.bind == "127.0.0.1:18009" {
		bind = v
	}
	token := serveFlags.token
	if v := os.Getenv("NINEBOT_SERVE_TOKEN"); v != "" {
		token = v
	}
	client := api.New(hostsOrDefaults(), cfg, tok)
	srv := proxy.NewServer(client, d, serveFlags.quiet, token)
	if !serveFlags.quiet {
		log.Printf("ninecli serve listening on %s (config: %s)", bind, d)
		if token == "" && !proxy.IsLoopback(bind) {
			log.Printf("warning: binding non-loopback without --token")
		}
	}
	if !serveFlags.quiet {
		return http.ListenAndServe(bind, srv)
	}
	log.SetOutput(os.Stderr)
	return http.ListenAndServe(bind, srv)
}
