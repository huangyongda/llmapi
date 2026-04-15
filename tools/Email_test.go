package tools

import (
	"testing"
)

func TestEmailConfigNotInit(t *testing.T) {
	// 测试配置未初始化的情况
	err := Email("test@example.com", "Subject", "Body")
	if err == nil {
		t.Error("预期返回错误，但返回了 nil")
	}
	if err.Error() != "邮件配置未初始化" {
		t.Errorf("预期错误信息 '邮件配置未初始化'，实际: %s", err.Error())
	}
}

func TestEmailNotEnabled(t *testing.T) {
	// 测试未启用的情况
	InitEmail(&EmailConfig{
		Enabled: false,
	})
	err := Email("test@example.com", "Subject", "Body")
	if err == nil {
		t.Error("预期返回错误，但返回了 nil")
	}
	if err.Error() != "邮件功能未启用" {
		t.Errorf("预期错误信息 '邮件功能未启用'，实际: %s", err.Error())
	}
}

func TestEmailEmptyRecipient(t *testing.T) {
	// 测试收件人为空的情况
	InitEmail(&EmailConfig{
		Enabled:  true,
		Host:     "smtp.163.com",
		Port:     465,
		Username: "a3421675@163.com",
		Password: "SLdXPzcrXDuvbJAB",
		From:     "a3421675@163.com",
		UseSSL:   true,
	})
	err := Email("", "Subject", "Body")
	if err == nil {
		t.Error("预期返回错误，但返回了 nil")
	}
	if err.Error() != "收件人不能为空" {
		t.Errorf("预期错误信息 '收件人不能为空'，实际: %s", err.Error())
	}
}

func TestEmailConfigIncomplete(t *testing.T) {
	// 测试配置不完整的情况 - Host 为空
	InitEmail(&EmailConfig{
		Enabled:  true,
		Host:     "",
		Port:     465,
		Username: "test@163.com",
		Password: "password",
		From:     "test@163.com",
		UseSSL:   true,
	})
	err := Email("test@example.com", "Subject", "Body")
	if err == nil {
		t.Error("预期返回错误，但返回了 nil")
	}
	if err.Error() != "邮件配置不完整" {
		t.Errorf("预期错误信息 '邮件配置不完整'，实际: %s", err.Error())
	}
}

func TestEmailConfigNotInitialized(t *testing.T) {
	// 测试配置未初始化的情况（重现置）
	emailConfig = nil
	err := Email("test@example.com", "Subject", "Body")
	if err == nil {
		t.Error("预期返回错误，但返回了 nil")
	}
	if err.Error() != "邮件配置未初始化" {
		t.Errorf("预期错误信息 '邮件配置未初始化'，实际: %s", err.Error())
	}
}

func TestGetEmailConfig(t *testing.T) {
	// 测试获取配置
	config := &EmailConfig{
		Enabled:  true,
		Host:     "smtp.163.com",
		Port:     465,
		Username: "a3421675@163.com",
		Password: "SLdXPzcrXDuvbJAB",
		From:     "a3421675@163.com",
		FromName: "nengpa.com API",
		UseSSL:   true,
	}
	InitEmail(config)

	retrieved := GetEmailConfig()
	if retrieved == nil {
		t.Error("GetEmailConfig 返回了 nil")
	}
	if retrieved.Host != "smtp.163.com" {
		t.Errorf("期望 Host 为 smtp.163.com，实际: %s", retrieved.Host)
	}
}

func TestSendEmail(t *testing.T) {
	// 初始化邮件配置
	InitEmail(&EmailConfig{
		Enabled:  true,
		Host:     "smtp.163.com",
		Port:     465,
		Username: "a3421675@163.com",
		Password: "SLdXPzcrXDuvbJAB",
		From:     "a3421675@163.com",
		FromName: "nengpa.com API",
		UseSSL:   true,
		Timeout:  30,
	})

	// 发送测试邮件
	err := Email("122402509@qq.com", "测试邮件", "这是一封来自 nengpa.com API 的测试邮件")
	if err != nil {
		t.Errorf("发送邮件失败: %v", err)
	}
	t.Log("邮件发送成功")
}
