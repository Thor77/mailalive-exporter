package main

import "github.com/BurntSushi/toml"

type MailgunConfig struct {
	APIKey string
	Domain string
	To     string
}

type IMAPConfig struct {
	Addr     string
	Username string
	Password string
}

type Config struct {
	Mailgun MailgunConfig
	IMAP    IMAPConfig
	Addr    string
}

func ParseConfig(path string) (Config, error) {
	c := Config{}
	_, err := toml.DecodeFile(path, &c)

	if c.Addr == "" {
		c.Addr = ":8080"
	}

	return c, err
}
