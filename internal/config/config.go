package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config estructura principal de configuración
type Config struct {
	FastAGI  FastAGIConfig  `yaml:"fastagi"`
	AMI      AMIConfig      `yaml:"ami"`
	API      APIConfig      `yaml:"api"`
	Database DatabaseConfig `yaml:"database"`
	Asterisk AsteriskConfig `yaml:"asterisk"`
	Log      LogConfig      `yaml:"log"`
}

type FastAGIConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type AMIConfig struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	Username          string `yaml:"username"`
	Secret            string `yaml:"secret"`
	ReconnectInterval int    `yaml:"reconnect_interval"`
}

type APIConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	EnableCORS bool   `yaml:"enable_cors"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type AsteriskConfig struct {
	SoundPath       string `yaml:"sound_path"`
	DefaultContext  string `yaml:"default_context"`
	OutboundContext string `yaml:"outbound_context"`
	MaxCPS          int    `yaml:"max_cps"` // Límite de llamadas por segundo
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load carga la configuración desde archivo YAML
func Load(path string) (*Config, error) {
	// Intentar leer el archivo
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo de configuración: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parseando YAML: %w", err)
	}

	// Permitir sobrescribir con variables de entorno
	overrideWithEnv(&cfg)

	return &cfg, nil
}

// overrideWithEnv permite sobrescribir configuración con variables de entorno
func overrideWithEnv(cfg *Config) {
	if v := os.Getenv("APICALL_AMI_USERNAME"); v != "" {
		cfg.AMI.Username = v
	}
	if v := os.Getenv("APICALL_AMI_SECRET"); v != "" {
		cfg.AMI.Secret = v
	}
	if v := os.Getenv("APICALL_DB_USERNAME"); v != "" {
		cfg.Database.Username = v
	}
	if v := os.Getenv("APICALL_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("APICALL_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("APICALL_DB_DATABASE"); v != "" {
		cfg.Database.Database = v
	}
}

// Address devuelve la dirección completa del servidor FastAGI
func (f FastAGIConfig) Address() string {
	return fmt.Sprintf("%s:%d", f.Host, f.Port)
}

// Address devuelve la dirección completa del servidor API
func (a APIConfig) Address() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// Address devuelve la dirección completa del servidor AMI
func (a AMIConfig) Address() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// DSN devuelve el Data Source Name para MySQL
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		d.Username, d.Password, d.Host, d.Port, d.Database)
}
