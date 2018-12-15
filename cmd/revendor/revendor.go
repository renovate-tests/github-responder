package main

import (
	"context"
	"github.com/google/go-github/github"
	"github.com/rs/zerolog/log"
	"path"
)

func revendor(ctx context.Context, eventType, deliveryID string, payload []byte) {
	log := log.Ctx(ctx).With().Str("action", "revendor").Logger()
	log.Info().Int("size", len(payload)).Msg("Received Event")
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse event")
	}
	var push *github.PushEvent
	switch event := event.(type) {
	default:
		log.Warn().Msg("Ignoring event")
	case *github.PushEvent:
		for _, commit := range event.Commits {
			for _, mod := range commit.Modified {
				log.Info().Str("modified", mod).Msg("Modified")
				fn := path.Base(mod)
				if fn == "go.mod" {
					log.Info().Msg("Will need to revendor!")
					push = event
				}
			}
			for _, mod := range commit.Added {
				log.Info().Str("modified", mod).Msg("Modified")
				fn := path.Base(mod)
				if fn == "go.mod" {
					log.Info().Msg("Will need to revendor!")
					push = event
				}
			}
		}
	}

	if push != nil {
		log.Info().Str("ref", *push.Ref).Msg("")
	}
}
