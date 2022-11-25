package wrench

type Config struct {
	// ExternalURL where Wrench is hosted, if any.
	ExternalURL string

	// Address serve on, e.g. ":443" or ":80".
	//
	// Disabled if an empty string.
	Address string

	// (optional) Discord bot token. See README.md for details on how to create this.
	//
	// Disabled if an empty string.
	DiscordBotToken string

	// (required if DiscordBotToken is set) Discord guild/server ID to operate in.
	//
	// Find this via User Settings -> Advanced -> Enabled developer mode, then right-click on any
	// server and Copy ID)
	DiscordGuildID string

	// (optional) Discord channel name for Wrench to send messages in. Defaults to "wrench"
	DiscordChannel string

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

	// (optional) Generic secret used to authenticate with this server. Any arbitrary string.
	Secret string `toml:"Secret,omitempty"`

	// (optional) Act as a runner, connecting to the root Wrench server specified in ExternalURL.
	Runner string `toml:"Runner,omitempty"`
}
