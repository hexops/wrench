package wrench

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
)

type ModeType string

const (
	ModeWrench ModeType = "wrench"
	ModePkg    ModeType = "pkg"
	ModeZig    ModeType = "zig"
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

	// When PkgProxy=true, disable any HTTP requests to pkg.machengine.org
	//
	// Note: setting this means your mirror may not be able to get Mach nominated
	// versions, because ziglang.org purges them after some time.
	PkgProxyDisableMachMirror bool `toml:"PkgProxyDisableMachMirror,omitempty"`

	// Mode to operate in, one of:
	//
	// * "wrench" -> https://wrench.machengine.org - custom CI system, etc.
	// * "pkg" -> https://pkg.machengine.org - package mirror and Zig download mirror
	// * "zig" -> Zig download mirror only (subset of pkg.machengine.org)
	Mode string `toml:"Mode,omitempty"`

	// (optional) Directory for caching LetsEncrypt certificates
	LetsEncryptCacheDir string `toml:"LetsEncryptCacheDir,omitempty"`

	// (optional) Email to use for LetsEncrypt notifications
	LetsEncryptEmail string `toml:"LetsEncryptEmail,omitempty"`

	// Where Wrench should store its data, cofiguration, etc. Defaults to the directory containing
	// this config file.
	WrenchDir string `toml:"WrenchDir,omitempty"`

	// All options below here are only used in "wrench" mode.

	// (optional) Discord bot token. See README.md for details on how to create this.
	//
	// Disabled if an empty string.
	//
	// Only used in "wrench" mode.
	DiscordBotToken string `toml:"DiscordBotToken,omitempty"`

	// (required if DiscordBotToken is set) Discord guild/server ID to operate in.
	//
	// Find this via User Settings -> Advanced -> Enabled developer mode, then right-click on any
	// server and Copy ID)
	//
	// Only used in "wrench" mode.
	DiscordGuildID string `toml:"DiscordGuildID,omitempty"`

	// (optional) Discord channel name for Wrench to send messages in. Defaults to "wrench"
	//
	// Only used in "wrench" mode.
	DiscordChannel string `toml:"DiscordChannel,omitempty"`

	// (optional) Discord channel name for Wrench to relay all Discord messages to. Defaults to "disabled"
	//
	// Only used in "wrench" mode.
	ActivityChannel string `toml:"ActivityChannel,omitempty"`

	// (optional) When specified, this is an arbitrary secret of your choosing which can be used to
	// send GitHub webhook events from the github.com/hexops/wrench repository itself to Wrench. It
	// will respond to these by recompiling and launching itself:
	//
	// The webhook URL should be: /webhook/github/self
	//
	// Only used in "wrench" mode.
	GitHubWebHookSecret string `toml:"GitHubWebHookSecret,omitempty"`

	// (optional) When specified Wrench can send PRs and assist with GitHub.
	//
	// Only applicable if running as the Wrench server.
	//
	// Only used in "wrench" mode.
	GitHubAccessToken string `toml:"GitHubAccessToken,omitempty"`

	// (optional) When specified wrench runners can push to Git using this configuration.
	// It is suggested to use a limited push access token for the password since it will
	// be distributed to all runners.
	//
	// Only applicable if running as the Wrench server.
	//
	// Only used in "wrench" mode.
	GitPushUsername    string `toml:"GitPushUsername,omitempty"`
	GitPushPassword    string `toml:"GitPushPassword,omitempty"`
	GitConfigUserName  string `toml:"GitConfigUserName,omitempty"`
	GitConfigUserEmail string `toml:"GitConfigUserEmail,omitempty"`

	// (optional) Generic secret used to authenticate with this server. Any arbitrary string.
	//
	// Only used in "wrench" mode.
	Secret string `toml:"Secret,omitempty"`

	// (optional) Act as a runner, connecting to the root Wrench server specified in ExternalURL.
	//
	// Only used in "wrench" mode.
	Runner string `toml:"Runner,omitempty"`
}

func (c *Config) ModeType() ModeType {
	if c.Mode != "" {
		return ModeType(c.Mode)
	}
	if c.PkgProxy {
		return ModePkg
	}
	return ModeWrench
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
	if out.ActivityChannel == "" {
		out.ActivityChannel = "disabled"
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
