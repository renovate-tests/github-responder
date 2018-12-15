/*
The revendor command.

Responds to PRs where the `go.mod` file is modified, and makes sure that the
vendor directory is appropriately kept up-to-date.

*/
package main

import (
	"context"
	"fmt"
	"github.com/hairyhenderson/github-responder"
	"github.com/hairyhenderson/github-responder/autotls"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
)

var (
	printVer bool
	verbose  bool
	opts     responder.Config
	repo     string
)

const (
	ghtokName = "GITHUB_TOKEN"
)

func preRun(cmd *cobra.Command, args []string) error {
	if repo == "" {
		return errors.New("must provide repo")
	}

	token := os.Getenv(ghtokName)
	if token == "" {
		return errors.Errorf("GitHub API token missing - must set %s", ghtokName)
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return errors.Errorf("invalid repo %s - need 'owner/repo' form", repo)
	}
	opts.EnableTLS = true
	opts.Owner = parts[0]
	opts.Repo = parts[1]
	opts.CallbackURL = "https://" + opts.Domain + "/revendor-cb/" + uuid.NewV4().String()
	opts.GitHubToken = token
	opts.Events = []string{"push"}
	opts.HookSecret = "jellyfish"

	return nil
}

func newCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "revendor [ACTION]",
		Short: "Create and listen to GitHub WebHooks",
		Long: `Responds to PRs where the 'go.mod' file is modified, and makes sure that the
vendor directory is appropriately kept up-to-date.`,
		PreRunE: preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			ctx := context.Background()
			log.Printf("Starting responder with options %#v", opts)
			cleanup, err := responder.Start(ctx, opts, revendor)
			if err != nil {
				return err
			}
			log.Print("Responder started...")
			defer cleanup()

			return handleInterrupt(ctx)
		},
	}
	return rootCmd
}

func initFlags(command *cobra.Command) {
	command.Flags().SortFlags = false

	command.Flags().StringVarP(&repo, "repo", "r", "", "The GitHub repository to watch, in 'owner/repo' form")
	command.Flags().StringVar(&opts.HTTPAddress, "http", ":80", "Address to listen to for HTTP traffic.")
	command.Flags().StringVar(&opts.TLSAddress, "https", ":443", "Address to listen to for TLS traffic.")

	command.Flags().StringVarP(&opts.Domain, "domain", "d", "", "domain to serve - a cert will be acquired for this domain")
	command.Flags().StringVarP(&opts.Email, "email", "m", "", "Email used for registration and recovery contact.")
	command.Flags().BoolVarP(&opts.Accept, "accept-tos", "a", false, "By setting this flag to true you indicate that you accept the current Let's Encrypt terms of service.")
	command.Flags().StringVar(&opts.CAEndpoint, "ca", autotls.LetsEncryptProductionURL, "URL to certificate authority's ACME server directory. Change this to point to a different server for testing.")
	command.Flags().StringVar(&opts.StoragePath, "path", "", "Directory to use for storing data")
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	}

	command := newCmd()
	initFlags(command)
	if err := command.Execute(); err != nil {
		log.Error().Err(err).Msg(command.Name() + " failed")
		os.Exit(1)
	}
}

func handleInterrupt(ctx context.Context) (err error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case s := <-c:
		log.Debug().
			Str("signal", s.String()).
			Msg(fmt.Sprintf("received %v, shutting down gracefully...", s))
	case <-ctx.Done():
		err = ctx.Err()
		log.Error().
			Err(err).
			Msg("context cancelled")
	}
	return err
}
