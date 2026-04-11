package config

import (
	"fmt"
	"llmapi/tools"
	"math/rand/v2"
	"sync"

	"github.com/gin-gonic/gin"
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
	Provider     string            `mapstructure:"provider"`
	MaxRetrys    int               `mapstructure:"max_retries"`
	APIURL       string            `mapstructure:"api_url"`
	APIKeys      []string          `mapstructure:"api_keys"`
	APIWeights   []float32         `mapstructure:"api_weights"`
	APIKeys2     []string          `mapstructure:"api_keys2"`
	APIWeights2  []float32         `mapstructure:"api_weights2"`
	GlmAPIKeys   []string          `mapstructure:"glm_api_keys"`
	Timeout      int               `mapstructure:"timeout"`
	ProxyURL     string            `mapstructure:"proxy_url"`
	ModelMapping map[string]string `mapstructure:"model_mapping"`

	// 内部状态
	keyIndex    int
	keyMutex    sync.Mutex
	keyUseCount map[string]int // 记录每个key当前被使用的数量
	useCountMu  sync.Mutex
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

// GetNextAPIKey 轮询获取下一个 API key，使用计数+1
func (c *LLMConfig) GetNextAPIKey(con *gin.Context) string {
	key := ""
	level, _ := con.Get("level")
	if level == 1 {
		key = tools.Selector.Select()
	} else {
		key = tools.Selector2.Select()
	}

	gmlModel, _ := con.Get("gmlModel")
	useGlm, _ := con.Get("UseGlm")
	if gmlModel == true && useGlm == 1 {
		keys := AppConfig.LLM.GlmAPIKeys
		if len(keys) > 0 {
			randomIndex := rand.IntN(len(keys))
			key = keys[randomIndex]
		}
	}

	// if len(c.APIKeys) == 0 {
	// 	return ""
	// }

	// c.keyMutex.Lock()
	// defer c.keyMutex.Unlock()

	// key := c.APIKeys[c.keyIndex]
	// c.keyIndex = (c.keyIndex + 1) % len(c.APIKeys)

	// 增加该key的使用计数
	c.useCountMu.Lock()
	if c.keyUseCount == nil {
		c.keyUseCount = make(map[string]int)
	}
	c.keyUseCount[key]++
	c.useCountMu.Unlock()

	return key
}

// ReleaseAPIKey 释放指定key，使用计数-1
func (c *LLMConfig) ReleaseAPIKey(key string) {
	c.useCountMu.Lock()
	defer c.useCountMu.Unlock()
	if c.keyUseCount == nil {
		c.keyUseCount = make(map[string]int)
	}
	if c.keyUseCount[key] > 0 {
		c.keyUseCount[key]--
	}
}

// GetCurUseInfo 获取所有key当前并发使用的个数
func (c *LLMConfig) GetCurUseInfo() map[string]int {
	c.useCountMu.Lock()
	defer c.useCountMu.Unlock()
	result := make(map[string]int)
	for k, v := range c.keyUseCount {
		result[k] = v
	}
	return result
}

func (c *LLMConfig) GetKeyUseInfo(key string) int {
	c.useCountMu.Lock()
	defer c.useCountMu.Unlock()
	return c.keyUseCount[key]
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
