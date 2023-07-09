package wrench

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

type Config struct {
	// ExternalURL where Wrench is hosted, if any.
	ExternalURL string

	// Address serve on, e.g. ":443" or ":80".
	//
	// Disabled if an empty string.
	Address string `toml:"Address,omitempty"`

	// Act as a Zig package proxy like pkg.machengine.org, instead of as a regular wrench server.
	PkgProxy bool `toml:"PkgProxy,omitempty"`

	// (optional) Discord bot token. See README.md for details on how to create this.
	//
	// Disabled if an empty string.
	DiscordBotToken string `toml:"DiscordBotToken,omitempty"`

	// (required if DiscordBotToken is set) Discord guild/server ID to operate in.
	//
	// Find this via User Settings -> Advanced -> Enabled developer mode, then right-click on any
	// server and Copy ID)
	DiscordGuildID string `toml:"DiscordGuildID,omitempty"`

	// (optional) Discord channel name for Wrench to send messages in. Defaults to "wrench"
	DiscordChannel string `toml:"DiscordChannel,omitempty"`

	// (optional) Directory for caching LetsEncrypt certificates
	LetsEncryptCacheDir string `toml:"LetsEncryptCacheDir,omitempty"`

	// (optional) Email to use for LetsEncrypt notifications
	LetsEncryptEmail string `toml:"LetsEncryptEmail,omitempty"`

	// (optional) When specified, this is an arbitrary secret of your choosing which can be used to
	// send GitHub webhook events from the github.com/hexops/wrench repository itself to Wrench. It
	// will respond to these by recompiling and launching itself:
	//
	// The webhook URL should be: /webhook/github/self
	//
	GitHubWebHookSecret string `toml:"GitHubWebHookSecret,omitempty"`

	// (optional) When specified Wrench can send PRs and assist with GitHub.
	//
	// Only applicable if running as the Wrench server.
	GitHubAccessToken string `toml:"GitHubAccessToken,omitempty"`

	// (optional) When specified wrench runners can push to Git using this configuration.
	// It is suggested to use a limited push access token for the password since it will
	// be distributed to all runners.
	//
	// Only applicable if running as the Wrench server.
	GitPushUsername    string `toml:"GitPushUsername,omitempty"`
	GitPushPassword    string `toml:"GitPushPassword,omitempty"`
	GitConfigUserName  string `toml:"GitConfigUserName,omitempty"`
	GitConfigUserEmail string `toml:"GitConfigUserEmail,omitempty"`

	// (optional) Generic secret used to authenticate with this server. Any arbitrary string.
	Secret string `toml:"Secret,omitempty"`

	// (optional) Act as a runner, connecting to the root Wrench server specified in ExternalURL.
	Runner string `toml:"Runner,omitempty"`

	// Where Wrench should store its data, cofiguration, etc. Defaults to the directory containing
	// this config file.
	WrenchDir string `toml:"WrenchDir,omitempty"`
}

func (c *Config) LogFilePath() string {
	return filepath.Join(c.WrenchDir, "logs")
}

func (c *Config) WriteTo(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}
	f, err := os.Create(file)
	if err != nil {
		return errors.Wrap(err, "Create")
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return errors.Wrap(enc.Encode(c), "Encode")
}

func LoadConfig(file string, out *Config) error {
	_, err := toml.DecodeFile(file, out)
	if err != nil {
		return err
	}
	// Default config values.
	if out.LetsEncryptCacheDir == "" {
		out.LetsEncryptCacheDir = "cache"
	}
	if out.DiscordChannel == "" {
		out.DiscordChannel = "wrench"
	}
	if out.WrenchDir == "" {
		out.WrenchDir, err = filepath.Abs(filepath.Dir(file))
		if err != nil {
			return errors.Wrap(err, "Abs")
		}
	}
	return nil
}

func Client(configFile string) (*api.Client, error) {
	var cfg Config
	if err := LoadConfig(configFile, &cfg); err != nil {
		return nil, err
	}
	return &api.Client{
		URL:    cfg.ExternalURL,
		Secret: cfg.Secret,
	}, nil
}
