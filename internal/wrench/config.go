package wrench

type Config struct {
	// ExternalURL where Wrench is hosted, if any.
	ExternalURL string

	// Address serve on, e.g. ":443" or ":80".
	//
	// Disabled if an empty string.
	Address string

	// Discord bot token. See README.md for details on how to create this.
	//
	// Disabled if an empty string.
	DiscordBotToken string

	// Directory for caching LetsEncrypt certificates
	LetsEncryptCacheDir string `toml:"omitempty"`
}
