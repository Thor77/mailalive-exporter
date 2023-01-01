package main

import (
	"time"

	"github.com/BurntSushi/toml"
)

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
	Mailgun         MailgunConfig
	IMAP            IMAPConfig
	Addr            string
	CacheTTL        time.Duration
	MessageInterval time.Duration
}

func ParseConfig(path string) (Config, error) {
	c := Config{}
	_, err := toml.DecodeFile(path, &c)

	if c.Addr == "" {
		c.Addr = ":8080"
	}

	if c.CacheTTL == 0 {
		c.CacheTTL = time.Duration(5 * time.Minute)
	}

	if c.MessageInterval == 0 {
		c.MessageInterval = time.Duration(1 * time.Hour)
	}

	return c, err
}
