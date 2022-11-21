package wrench

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/bwmarrin/discordgo"
	"github.com/hexops/wrench/internal/errors"
)

type Bot struct {
	ConfigFile string
	Config     *Config

	store             *Store
	discordSession    *discordgo.Session
	webHookGitHubSelf sync.Mutex
}

func (b *Bot) loadConfig() error {
	if b.Config == nil {
		if b.ConfigFile != "" {
			_, err := toml.DecodeFile(b.ConfigFile, &b.Config)
			if errors.Is(err, os.ErrNotExist) {
				_, err := toml.DecodeFile("../wrench-private/config.toml", &b.Config)
				return err
			}
			if err != nil {
				return err
			}
		} else {
			return errors.New("expected Config or ConfigFile to be specified")
		}
	}
	if b.Config.LetsEncryptCacheDir == "" {
		b.Config.LetsEncryptCacheDir = "cache"
	}
	if b.Config.DiscordChannel == "" {
		b.Config.DiscordChannel = "wrench"
	}
	return nil
}

func (b *Bot) logf(format string, v ...any) {
	b.idLogf("general", format, v...)
}

func (b *Bot) idLogf(id, format string, v ...any) {
	log.Printf(format, v...)
	b.store.Log(context.Background(), id, fmt.Sprintf(format, v...))
}

func (b *Bot) idWriter(id string) io.Writer {
	return writerFunc(func(p []byte) (n int, err error) {
		b.idLogf(id, "%s", p)
		return len(p), nil
	})
}

type writerFunc func(p []byte) (n int, err error)

func (w writerFunc) Write(p []byte) (n int, err error) {
	return w(p)
}

func (b *Bot) Start() error {
	var err error
	b.store, err = OpenStore("wrench.db")
	if err != nil {
		return errors.Wrap(err, "OpenStore")
	}

	if err := b.loadConfig(); err != nil {
		return errors.Wrap(err, "loading config")
	}
	if err := b.discordStart(); err != nil {
		return errors.Wrap(err, "discord")
	}
	if err := b.httpStart(); err != nil {
		return errors.Wrap(err, "http")
	}

	b.discord("<:wrench:1013705194736975883> A new day, a new me. Just rebuilt myself!")

	// Wait here until CTRL-C or other term signal is received.
	b.logf("Running (press CTRL-C to exit.)")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc

	return errors.Wrap(b.Stop(), "Stop")
}

func (b *Bot) Stop() error {
	if err := b.discordStop(); err != nil {
		return errors.Wrap(err, "discord")
	}
	if err := b.httpStop(); err != nil {
		return errors.Wrap(err, "http")
	}
	if err := b.store.Close(); err != nil {
		return errors.Wrap(err, "Store.Close")
	}
	return nil
}
