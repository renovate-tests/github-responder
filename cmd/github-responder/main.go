/*
The github-responder command

*/
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"

	"github.com/hairyhenderson/github-responder"
	"github.com/hairyhenderson/github-responder/autotls"
	"github.com/hairyhenderson/github-responder/version"

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
	if verbose {
		setVerboseLogging()
	}

	if repo == "" {
		return errors.New("must provide repo")
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return errors.Errorf("invalid repo %s - need 'owner/repo' form", repo)
	}
	opts.Owner = parts[0]
	opts.Repo = parts[1]

	if opts.CallbackURL == "" {
		u := uuid.NewV4()
		callbackURL := ""
		if opts.EnableTLS {
			callbackURL = "https://"
		} else {
			callbackURL = "http://"
		}
		callbackURL = callbackURL + opts.Domain + "/gh-callback/" + u.String()
		opts.CallbackURL = callbackURL
	}

	if opts.HookSecret == "" {
		opts.HookSecret = fmt.Sprintf("%x", rand.Int63())
	}

	if opts.GitHubToken == "" {
		token := os.Getenv(ghtokName)
		if token == "" {
			return errors.Errorf("GitHub API token missing - must set %s", ghtokName)
		}

		opts.GitHubToken = token
	}

	return nil
}

func printVersion(name string) {
	fmt.Printf("%s version %s\n", name, version.Version)
}

func newCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "github-responder [ACTION]",
		Short: "Create and listen to GitHub WebHooks",
		Example: `  Run ./handle_event.sh every time a webhook event is received:

  $ github-responder -a -d example.com -e me@example.com ./handle_event.sh`,
		PreRunE: preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVer {
				printVersion(cmd.Name())
				return nil
			}
			log.Debug().
				Str("version", version.Version).
				Str("commit", version.GitCommit).
				Str("built", version.BuildDate).
				Msg("github-responder")
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			var action func(string, string, []byte)
			if len(args) > 0 {
				action = execArgs(args...)
			} else {
				log.Info().Msg("No action command given, will perform default")
				action = defaultAction
			}

			ctx := context.Background()
			log.Printf("Starting responder with options %#v", opts)
			cleanup, err := responder.Start(ctx, opts, action)
			if err != nil {
				return err
			}
			log.Print("Responder started...")
			defer cleanup()

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
		},
		Args: cobra.ArbitraryArgs,
	}
	return rootCmd
}

func initFlags(command *cobra.Command) {
	command.Flags().SortFlags = false

	command.Flags().StringVarP(&repo, "repo", "r", "", "The GitHub repository to watch, in 'owner/repo' form")
	command.Flags().StringVar(&opts.CallbackURL, "callback", "", "The WebHook Callback URL. If left blank, one will be generated for you.")
	command.Flags().StringArrayVarP(&opts.Events, "events", "e", []string{"*"}, "The GitHub event types to listen for. See https://developer.github.com/webhooks/#events for the full list.")

	command.Flags().StringVar(&opts.HTTPAddress, "http", ":80", "Address to listen to for HTTP traffic.")
	command.Flags().StringVar(&opts.TLSAddress, "https", ":443", "Address to listen to for TLS traffic.")

	command.Flags().BoolVar(&opts.EnableTLS, "tls", true, "Enable automatic TLS negotiation")
	command.Flags().StringVarP(&opts.Domain, "domain", "d", "", "domain to serve - a cert will be acquired for this domain")
	command.Flags().StringVarP(&opts.Email, "email", "m", "", "Email used for registration and recovery contact.")
	command.Flags().BoolVarP(&opts.Accept, "accept-tos", "a", false, "By setting this flag to true you indicate that you accept the current Let's Encrypt terms of service.")
	command.Flags().StringVar(&opts.CAEndpoint, "ca", autotls.LetsEncryptProductionURL, "URL to certificate authority's ACME server directory. Change this to point to a different server for testing.")
	command.Flags().StringVar(&opts.StoragePath, "path", "", "Directory to use for storing data")

	command.Flags().BoolVarP(&verbose, "verbose", "V", false, "Output extra logs")
	command.Flags().BoolVarP(&printVer, "version", "v", false, "Print the version")
}

func main() {
	initLogger()

	command := newCmd()
	initFlags(command)
	if err := command.Execute(); err != nil {
		log.Error().Err(err).Msg(command.Name() + " failed")
		os.Exit(1)
	}
}
