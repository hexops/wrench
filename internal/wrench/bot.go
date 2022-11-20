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
	if b.Config != nil {
		return nil
	}
	if b.ConfigFile != "" {
		_, err := toml.DecodeFile(b.ConfigFile, &b.Config)
		return err
	}
	return errors.New("expected Config or ConfigFile to be specified")
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
	return nil
}
