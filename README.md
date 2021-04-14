# Tgbot-YouTube-Notifier [WIP]

## Requirements

### Setting
A setting `JSON` file is also required.
```json
{
    "host": "<Hostname>",
    "bot_token": "<Bot Token>",
    "database": "<Database Path>",
    "yt_api_key": "<YouTube API Key>"
}
```

### Bot Token
Contact [BotFather](https://t.me/BotFather) to create your own bot, and get the bot token.

### Certification
[Here](https://core.telegram.org/bots/webhooks#the-short-version) is the requirements of the server.

If you have domain name, you can simplely use [Let’s Encrypt](https://letsencrypt.org/) to get your certification.

Otherwise, you can follow [this](https://core.telegram.org/bots/self-signed) tutorial to get self-signed certification.

#### Apply Certification
You have two ways to setup certification.

1. Use web server (e.g. Nginx) to listen & proxy pass tgbot request to server port
2. Use provide certification file path `ssl_cert` & `ssl_key` in setting file and use standalone server

For the 2nd way, start server with parameter `--use_ssl=True`.