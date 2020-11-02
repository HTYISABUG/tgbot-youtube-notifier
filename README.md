# Tgbot-YouTube-Notifier [WIP]

## Requirements

### Libraries

Following are required libraries.
```shell
go get github.com/dpup/gohubbub
go get github.com/bitly/go-simplejson
go get github.com/mattn/go-sqlite3
```

### Setting

A setting `JSON` file is also required.
```json
{
    "host": "<hostname>",
    "bot_token": "<bot token>",
    "ssl_cert": "<cert file path>",
    "ssl_key": "<key file path>",
    "database": "<database path>"
}
```

### Bot Token
Contact [BotFather](https://t.me/BotFather) to create your own bot, and get the bot token.

### Certification

[Here](https://core.telegram.org/bots/webhooks#the-short-version) is the requirements of the server.

If you have domain name, you can simplely use [Let’s Encrypt](https://letsencrypt.org/) to get your certification.

Otherwise, you canfollow [this](https://core.telegram.org/bots/self-signed) tutorial to get self-signed certification.
