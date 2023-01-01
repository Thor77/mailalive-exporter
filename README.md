# mailalive-exporter
End-to-End monitoring for mail delivery using MailGun

This exporter will periodically send mails using MailGun and then use IMAP to check whether they were delivered successfully exporting metrics about the age of the last message and the delivery delay.

## Installation

## Configuration
```
addr = ":8080"

[mailgun]
apikey = "<mailgun api key>"
domain = "<mailgun domain>"
to = "<to address for mailgun api call>"

[imap]
addr = "<imap server addr>"
username = "<imap username>"
password = "<imap password>"
```

The path to the configuration file can be passed as the first argument, if not specified `config.toml` from the current directory is used.

## References
Implementation of [mailalive](https://github.com/Thor77/mailalive) as a Prometheus Exporter
