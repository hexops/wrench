package wrench

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/kardianos/service"
)

type Bot struct {
	ConfigFile string
	Config     *Config

	started              bool
	store                *Store
	discordSession       *discordgo.Session
	discordCommandHelp   [][2]string
	discordCommands      map[string]func(...string) string
	discordCommandsEmbed map[string]func(...string) *discordgo.MessageEmbed
	runner               *api.Client
	webHookGitHubSelf    sync.Mutex
}

func (b *Bot) loadConfig() error {
	if b.Config == nil {
		if b.ConfigFile == "" {
			return errors.New("expected Config or ConfigFile to be specified")
		}
		b.Config = &Config{}
		return LoadConfig(b.ConfigFile, b.Config)
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

func (b *Bot) Start(s service.Service) error {
	logger, err := s.Logger(nil)
	if err != nil {
		return errors.Wrap(err, "Logger")
	}
	go func() {
		if err := b.run(s); err != nil {
			logger.Error(err)
			log.Fatal(err)
		}
	}()
	return nil
}

func (b *Bot) run(s service.Service) error {
	b.discordCommands = make(map[string]func(...string) string)
	b.discordCommandsEmbed = make(map[string]func(...string) *discordgo.MessageEmbed)

	exe, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "Executable")
	}
	dir := filepath.Dir(exe)

	b.store, err = OpenStore(filepath.Join(dir, "wrench.db") + "?_pragma=busy_timeout%3d10000")
	if err != nil {
		return errors.Wrap(err, "OpenStore")
	}

	if err := b.loadConfig(); err != nil {
		return errors.Wrap(err, "loading config")
	}
	if b.Config.Runner == "" {
		if err := b.discordStart(); err != nil {
			return errors.Wrap(err, "discord")
		}
		if err := b.httpStart(); err != nil {
			return errors.Wrap(err, "http")
		}
		b.registerCommands()
	} else {
		b.runnerStart()
	}

	b.started = true

	// Wait here until CTRL-C or other term signal is received.
	b.logf("Running (press CTRL-C to exit.)")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc

	fmt.Println("Interrupted, shutting down..")
	return errors.Wrap(s.Stop(), "Stop")
}

func (b *Bot) Stop(s service.Service) error {
	if !b.started {
		return nil
	}
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

func ServiceStatus(svc service.Service) (string, error) {
	status, err := svc.Status()
	if err != nil {
		return "", err
	}
	if status == service.StatusUnknown {
		return "unknown", nil
	} else if status == service.StatusRunning {
		return "running", nil
	} else if status == service.StatusStopped {
		return "stopped", nil
	}
	panic(fmt.Sprintf("unexpected status: %v", status))
}
