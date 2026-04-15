package tools

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// EmailConfig 邮件配置结构体
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"smtp_host"`
	Port     int    `yaml:"smtp_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
	UseTLS   bool   `yaml:"use_tls"`
	UseSSL   bool   `yaml:"use_ssl"`
	Timeout  int    `yaml:"timeout"`
}

// 包级变量（遵循现有模式）
var (
	emailConfig *EmailConfig
	emailMu     sync.RWMutex
)

// InitEmail 初始化邮件配置（在系统启动时调用）
func InitEmail(config *EmailConfig) {
	emailMu.Lock()
	defer emailMu.Unlock()
	emailConfig = config
}

// GetEmailConfig 获取当前邮件配置
func GetEmailConfig() *EmailConfig {
	emailMu.RLock()
	defer emailMu.RUnlock()
	return emailConfig
}

// Email 发送邮件函数
// 参数：
//   - to: 收件人邮箱地址
//   - subject: 邮件主题
//   - body: 邮件正文
//
// 返回：
//   - error: 发送失败时返回错误
func Email(to, subject, body string) error {
	emailMu.RLock()
	config := emailConfig
	emailMu.RUnlock()

	// 检查配置
	if config == nil {
		return errors.New("邮件配置未初始化")
	}
	if !config.Enabled {
		return errors.New("邮件功能未启用")
	}
	if to == "" {
		return errors.New("收件人不能为空")
	}
	if config.Host == "" || config.Username == "" || config.Password == "" || config.From == "" {
		return errors.New("邮件配置不完整")
	}

	// 构建邮件内容
	from := config.From
	if config.FromName != "" {
		from = config.FromName + " <" + config.From + ">"
	}

	var msg strings.Builder
	msg.WriteString("From: ")
	msg.WriteString(from)
	msg.WriteString("\r\n")
	msg.WriteString("To: ")
	msg.WriteString(to)
	msg.WriteString("\r\n")
	msg.WriteString("Subject: ")
	msg.WriteString(subject)
	msg.WriteString("\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// SMTP 地址
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// 认证
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// 发送邮件
	var err error
	if config.UseSSL || config.UseTLS {
		err = sendMailWithTLS(addr, auth, config.From, []string{to}, msg.String(), config.Timeout)
	} else {
		err = smtp.SendMail(addr, auth, config.From, []string{to}, []byte(msg.String()))
	}

	return err
}

// sendMailWithTLS 使用 TLS 发送邮件
func sendMailWithTLS(addr string, auth smtp.Auth, from string, to []string, msg string, timeout int) error {
	host := strings.Split(addr, ":")[0]

	// 建立 TCP 连接
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return err
	}

	// 对于 SSL 端口（465），需要先进行 TLS 握手
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}
	tlsConn := tls.Client(conn, tlsConfig)
	defer tlsConn.Close()

	// 设置超时
	tlsConn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		return err
	}
	defer client.Quit()

	// 认证
	if ok, _ := client.Extension("AUTH"); ok {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	// 设置发件人
	if err = client.Mail(from); err != nil {
		return err
	}

	// 设置收件人
	for _, t := range to {
		if err = client.Rcpt(t); err != nil {
			return err
		}
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = w.Close()
	return err
}