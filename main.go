package main

import (
	"fmt"
	"os"

	"github.com/pccr10001/llm-catpcha/mcp"
	"github.com/pccr10001/llm-catpcha/server"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "llm-captcha",
		Short: "LLM Captcha Solver Server",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP/WebSocket server",
		Run: func(cmd *cobra.Command, args []string) {
			port, _ := cmd.Flags().GetInt("port")
			srv := server.New()
			fmt.Printf("Starting server on :%d\n", port)
			if err := srv.Run(fmt.Sprintf(":%d", port)); err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP stdio server",
		Run: func(cmd *cobra.Command, args []string) {
			serverURL, _ := cmd.Flags().GetString("server")
			mcp.RunStdio(serverURL)
		},
	}
	mcpCmd.Flags().StringP("server", "s", "http://localhost:8080", "Captcha server URL")

	rootCmd.AddCommand(serveCmd, mcpCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
