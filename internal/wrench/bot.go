package wrench

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v48/github"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/kardianos/service"
)

type Bot struct {
	ConfigFile string
	Config     *Config

	started                    bool
	logFile                    *os.File
	store                      *Store
	github                     *github.Client
	discordSession             *discordgo.Session
	discordCommandHelp         [][2]string
	discordCommands            map[string]func(...string) string
	discordCommandsEmbed       map[string]func(...string) *discordgo.MessageEmbed
	discordCommandsEmbedSecure map[string]func(...string) *discordgo.MessageEmbed
	runner                     *api.Client
	rebuildSelfMu              sync.Mutex
	jobAcquire                 sync.Mutex
	schedule                   []ScheduledJob
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
	msg := fmt.Sprintf(format, v...)
	timeNow := time.Now().Format(time.RFC3339)
	for _, line := range strings.Split(msg, "\n") {
		fmt.Fprintf(b.logFile, "%s %s: %s\n", timeNow, id, line)
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", timeNow, id, line)
	}
	// May be called before DB is initialized.
	if b.store != nil {
		_ = b.store.Log(context.Background(), id, msg)
	}
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
	serviceLogger, _ := s.Logger(nil)
	if serviceLogger != nil && !service.Interactive() {
		_ = serviceLogger.Info("wrench service started")
	}

	go func() {
		if err := b.run(s); err != nil {
			if !service.Interactive() {
				b.logf("wrench service: FATAL: %s", err)
				if serviceLogger != nil {
					_ = serviceLogger.Error("wrench service: FATAL:", err)
				}
			}
			log.Fatal(err)
		}
	}()
	return nil
}

func (b *Bot) run(s service.Service) error {
	b.discordCommands = make(map[string]func(...string) string)
	b.discordCommandsEmbed = make(map[string]func(...string) *discordgo.MessageEmbed)
	b.discordCommandsEmbedSecure = make(map[string]func(...string) *discordgo.MessageEmbed)

	if err := b.loadConfig(); err != nil {
		return errors.Wrap(err, "loading config")
	}

	logFilePath := b.Config.LogFilePath()
	var err error
	b.logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("creating log file %s", logFilePath))
	}
	if !service.Interactive() {
		b.logf("wrench service: STARTED")
	}

	if b.Config.Runner == "" {
		if !b.Config.PkgProxy {
			b.store, err = OpenStore(filepath.Join(b.Config.WrenchDir, "wrench.db") + "?_pragma=busy_timeout%3d10000")
			if err != nil {
				return errors.Wrap(err, "OpenStore")
			}
			if err := b.githubStart(); err != nil {
				return errors.Wrap(err, "github")
			}
			if err := b.discordStart(); err != nil {
				return errors.Wrap(err, "discord")
			}
		}
		if err := b.httpStart(); err != nil {
			return errors.Wrap(err, "http")
		}
		if !b.Config.PkgProxy {
			if err := b.schedulerStart(); err != nil {
				return errors.Wrap(err, "scheduler")
			}
			b.registerCommands()
		}
	} else {
		if err := b.runnerStart(); err != nil {
			return errors.Wrap(err, "runner")
		}
	}

	b.started = true

	// Wait here until CTRL-C or other term signal is received.
	b.logf("Running (press CTRL-C to exit.)")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc

	b.logf("Interrupted, shutting down..")

	return errors.Wrap(b.stop(), "stop")
}

func (b *Bot) Stop(s service.Service) error {
	serviceLogger, _ := s.Logger(nil)
	if serviceLogger != nil && !service.Interactive() {
		_ = serviceLogger.Info("wrench service stopped")
	}
	return b.stop()
}

func (b *Bot) stop() error {
	if !b.started {
		return nil
	}
	b.logFile.Close()
	if b.Config.Runner == "" {
		if err := b.githubStop(); err != nil {
			return errors.Wrap(err, "github")
		}
		if err := b.discordStop(); err != nil {
			return errors.Wrap(err, "discord")
		}
		if err := b.httpStop(); err != nil {
			return errors.Wrap(err, "http")
		}
		if b.store != nil {
			if err := b.store.Close(); err != nil {
				return errors.Wrap(err, "Store.Close")
			}
		}
		if err := b.schedulerStop(); err != nil {
			return errors.Wrap(err, "scheduler")
		}
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
