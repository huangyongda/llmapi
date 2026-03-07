package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Admin    AdminConfig    `mapstructure:"admin"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type LLMConfig struct {
	Provider      string            `mapstructure:"provider"`
	APIURL        string            `mapstructure:"api_url"`
	APIKeys       []string          `mapstructure:"api_keys"`
	Timeout       int               `mapstructure:"timeout"`
	ProxyURL      string            `mapstructure:"proxy_url"`
	ModelMapping  map[string]string `mapstructure:"model_mapping"`

	// 内部状态
	keyIndex int
	keyMutex sync.Mutex
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

var AppConfig *Config

func LoadConfig(path string) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) GetMySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.Username,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
	)
}

// GetNextAPIKey 轮询获取下一个 API key
func (c *LLMConfig) GetNextAPIKey() string {
	if len(c.APIKeys) == 0 {
		return ""
	}

	c.keyMutex.Lock()
	defer c.keyMutex.Unlock()

	key := c.APIKeys[c.keyIndex]
	c.keyIndex = (c.keyIndex + 1) % len(c.APIKeys)
	return key
}

// GetAPIKey 获取当前索引的 API key（不轮询）
func (c *LLMConfig) GetAPIKey() string {
	if len(c.APIKeys) == 0 {
		return ""
	}
	c.keyMutex.Lock()
	defer c.keyMutex.Unlock()
	return c.APIKeys[c.keyIndex]
}
