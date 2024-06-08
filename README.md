# [bot] wrench: let's fix this!

<img width="300px" align="left" src="https://raw.githubusercontent.com/hexops/media/b71e82ae9ea20c22a2eb3ab95d8ba48684635620/mach/wrench_rocket.svg">
<br>
<br>
<br>
<br>
<br>
<br>
<strong>Wrench</strong> is the name of the <a href="https://machengine.org">Mach engine</a> mascot, who also helps us automate and maintain Mach project development from time to time.
</div>
<br>
<br>
<br>
<br>
<br>
<br>

## Wrench in action

Join the `#wrench` channel in [the Mach discord](https://discord.gg/XNG3NZgCqp) to get an idea of what wrench can do.

## Issues

Issues are tracked in the [main Mach repository](https://github.com/hexops/mach/issues?q=is%3Aissue+is%3Aopen+label%3Awrench).

## Development

Wrench is written in Go (convenient due to its ecosystem for this kind of task); install the latest version of Go and `go test ./...` or `go install .` to install `wrench` into your `$GOBIN`.

### Discord bot testing

1. Navigate to the Discord [Applications page](https://discordapp.com/developers/applications/me)
2. Select one of your bot applications (you may need to create one for testing.)
3. Under **Settings**, click **Bot**. If needed create the bot.
5. Under the **Build-A-Bot** section, select **Click to Reveal Token**

In your `config.toml` enter the token:

```
DiscordBotToken = "SECRET"
```

To add the bot to a server:

* Under **OAuth2**, select **URL Generator**
* Select scopes:
  * Bot
* Select permissions:
  * Send messages
  * Create public threads
  * Create private threads
  * Send messages in threads
  * Send TTS messages
  * Embed links
  * Attach files
  * Read message history
  * Add reactions
  * Use slash commands

This will give you a URL you can visit to add the bot to your server.

## Installation

### Linux

1. Install Git `sudo apt-get install git` - ensure v2.39+
2. Download release binary
3. Move to `/usr/local/bin/wrench` and `chmod +x /usr/local/bin/wrench`
4. Run `sudo wrench setup` and follow instructions

### macOS

1. Ensure Git v2.39+ is installed (use `brew install git`)
2. Download release binary
3. Move to `$HOME/.bin/wrench` and `chmod +x $HOME/.bin/wrench`
4. Place `$HOME/.bin` on PATH if desired
5. Run `sudo wrench setup`

### Windows

1. Install Git: `winget install --id Git.Git -e --source winget` - ensure v2.39+
2. Download release binary, rename to `.exe`
3. Place at `C:/wrench.exe` or some other location. Put on PATH if desired.
4. In admin terminal run `wrench.exe setup`

## Run your own ziglang.org/download mirror

If you want to mirror https://ziglang.org/download on-demand, similar to what https://pkg.machengine.org does,
you may do so using e.g. this `config.toml` with `Mode = "zig"` which disables all other Wrench functionality so it only mirrors Zig downloads:

```toml
# Note: data will be written in a directory relative to this config file.
Mode = "zig"

# HTTP configuration
ExternalURL = "http://foobar.com"
Address = ":80"

# HTTPS configuration (optional, uses LetsEncrypt)
#ExternalURL = "https://foobar.com"
#Address = ":443"
#LetsEncryptEmail = "foo@bar.com"
```

Wrench will save data relative to that config file, so generally you should put that `config.toml` into e.g. a `wrench/` directory somewhere.

Running `wrench svc run` will start the server. Then you can fetch e.g.:

* http://localhost/
* http://localhost/zig/zig-linux-x86_64-0.13.0.tar.xz
* http://localhost/zig/index.json - a strict superset of https://ziglang.org/download/index.json

Downloads like http://localhost/zig/zig-linux-x86_64-0.13.0.tar.xz will be fetched on-demand from ziglang.org and then cached on the local filesystem forever after that.

http://localhost/zig/index.json is like https://ziglang.org/download/index.json with some small differences:

* It is fetched from ziglang.org once every 15 minutes and cached in-memory.
* Entries from https://machengine.org/zig/index.json are added so the index.json _additionally_ contains Mach [nominated Zig versions](https://machengine.org/about/nominated-zig/)
* `tarball` fields are rewritten to point to the configured `ExternalURL`

If you want to run Wrench as a system service, have it auto-start after reboot, etc. then you can e.g. put the config file in `/root/wrench/config.toml`, run `wrench svc install` as root to install the systemd service, use `wrench svc start` to start the service, and `wrench svc status` to see the status and log file locations.