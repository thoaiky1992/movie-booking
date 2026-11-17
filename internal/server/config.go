package server

import "time"

const defaultPort = 8080

type Config struct {
	Port           int
	IdleTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	AllowedOrigins []string
}

func DefaultConfig() Config {
	return Config{
		Port:           defaultPort,
		IdleTimeout:    time.Minute,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		AllowedOrigins: []string{"http://localhost:5173"},
	}
}

func (c Config) withDefaults() Config {
	defaults := DefaultConfig()
	if c.Port <= 0 {
		c.Port = defaults.Port
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = defaults.IdleTimeout
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = defaults.ReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = defaults.WriteTimeout
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = defaults.AllowedOrigins
	}
	return c
}
