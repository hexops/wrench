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
