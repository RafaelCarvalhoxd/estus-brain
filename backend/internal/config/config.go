// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	// Where uploaded documents are written. Relative paths resolve against
	// the working directory the binary runs from, which in development is
	// backend/ — so the archive sits inside the project by default.
	DocumentsDir string
	// AssistantDir holds the assistant's local state: the MCP token file and
	// the scratch directories the CLI engines run in.
	AssistantDir string
	// MCPToken, when set, is the bearer token MCP clients must present;
	// otherwise one is generated once and kept in AssistantDir.
	MCPToken string
	// Timezone is the owner's: the assistant reads "today" and bare times in it.
	Timezone string
	// MultiUser switches off the engines that run on the owner's own
	// Claude/ChatGPT subscription login — those are for personal use only.
	MultiUser bool
	// AppPassword is the owner's password: the frontend asks for it to log
	// in, and revealing a vault password asks for it again. Empty means no
	// password can be revealed.
	AppPassword string
}

func Load() (Config, error) {
	cfg := Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		DocumentsDir: getEnv("DOCUMENTS_DIR", "data/documents"),
		AssistantDir: getEnv("ASSISTANT_DIR", "data/assistant"),
		MCPToken:     os.Getenv("MCP_TOKEN"),
		Timezone:     getEnv("ASSISTANT_TZ", "America/Sao_Paulo"),
		MultiUser:    os.Getenv("ASSISTANT_MULTI_USER") == "true",
		AppPassword:  os.Getenv("APP_PASSWORD"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
