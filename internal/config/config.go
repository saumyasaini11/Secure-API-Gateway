package config

import (
    "log"
    "os"

    "gopkg.in/yaml.v3"
)

type RateLimit struct {
    Requests      int `yaml:"requests"`
    WindowSeconds int `yaml:"window_seconds"`
}

type Route struct {
    Path      string    `yaml:"path"`
    Backend   string    `yaml:"backend"`
    RateLimit RateLimit `yaml:"rate_limit"`
}

type RedisConfig struct {
    Addr     string `yaml:"addr"`
    Password string `yaml:"password"`
    DB       int    `yaml:"db"`
}

type JWTConfig struct {
    PrivateKeyPath string `yaml:"private_key_path"`
    PublicKeyPath  string `yaml:"public_key_path"`
    Issuer         string `yaml:"issuer"`
    TokenTTLMin    int    `yaml:"token_ttl_minutes"`
}

type ServerConfig struct {
    Port      int    `yaml:"port"`
    AdminPort int    `yaml:"admin_port"`
    Env       string `yaml:"env"`
}

type AuthConfig struct {
    TokenEndpoint string    `yaml:"token_endpoint"`
    RateLimit     RateLimit `yaml:"rate_limit"`
}

type Config struct {
    Server ServerConfig `yaml:"server"`
    Redis  RedisConfig  `yaml:"redis"`
    JWT    JWTConfig    `yaml:"jwt"`
    Routes []Route      `yaml:"routes"`
    Auth   AuthConfig   `yaml:"auth"`
}

func Load(path string) *Config {
    f, err := os.Open(path)
    if err != nil {
        log.Fatalf("Failed to open config file: %v", err)
    }
    defer f.Close()

    var cfg Config
    decoder := yaml.NewDecoder(f)
    if err := decoder.Decode(&cfg); err != nil {
        log.Fatalf("Failed to parse config file: %v", err)
    }

    // Allow environment variables to override YAML values.
    // This is used by docker-compose to point the gateway at the Redis
    // container (redis:6379) without requiring a separate config file.
    if addr := os.Getenv("REDIS_ADDR"); addr != "" {
        cfg.Redis.Addr = addr
    }

    if cfg.JWT.PrivateKeyPath == "" || cfg.JWT.PublicKeyPath == "" {
        log.Fatal("JWT key paths must be set in config.yaml")
    }
    if cfg.Redis.Addr == "" {
        log.Fatal("Redis address must be set in config.yaml")
    }
    if len(cfg.Routes) == 0 {
        log.Fatal("At least one route must be defined in config.yaml")
    }

    return &cfg
}
