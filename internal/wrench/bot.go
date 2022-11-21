package wrench

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/bwmarrin/discordgo"
	"github.com/hexops/wrench/internal/errors"
)

type Bot struct {
	ConfigFile string
	Config     *Config

	discordSession *discordgo.Session
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
	return nil
}

func (b *Bot) logf(format string, v ...any) {
	log.Printf(format, v...)
}

func (b *Bot) Start() error {
	if err := b.loadConfig(); err != nil {
		return errors.Wrap(err, "loading config")
	}
	if err := b.discordStart(); err != nil {
		return errors.Wrap(err, "discord")
	}
	if err := b.httpStart(); err != nil {
		return errors.Wrap(err, "http")
	}

	// Wait here until CTRL-C or other term signal is received.
	b.logf("Running (press CTRL-C to exit.)")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
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
	return nil
}
