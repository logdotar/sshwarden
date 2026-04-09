// Package alert 提供告警功能，包括邮件告警
package alert

import (
	"net/smtp"
	"strings"

	"go.uber.org/zap"
)

// EmailAlert 邮件告警结构体
//
// 字段说明：
// - SMTPHost: SMTP 服务器主机地址
// - SMTPPort: SMTP 服务器端口
// - Username: SMTP 用户名
// - Password: SMTP 密码
// - From: 发件人邮箱
// - To: 收件人邮箱列表
// - Subject: 邮件主题
// - logger: 日志器

type EmailAlert struct {
	SMTPHost string
	SMTPPort string
	Username string
	Password string
	From     string
	To       []string
	Subject  string
	logger   *zap.Logger
}

// NewEmailAlert 创建邮件告警实例
//
// 参数:
// - smtpHost: SMTP 服务器主机地址
// - smtpPort: SMTP 服务器端口
// - username: SMTP 用户名
// - password: SMTP 密码
// - from: 发件人邮箱
// - to: 收件人邮箱列表
// - subject: 邮件主题
// - logger: 日志器
//
// 返回值:
// - *EmailAlert: 邮件告警实例
func NewEmailAlert(smtpHost, smtpPort, username, password, from string, to []string, subject string, logger *zap.Logger) *EmailAlert {
	return &EmailAlert{
		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		Username: username,
		Password: password,
		From:     from,
		To:       to,
		Subject:  subject,
		logger:   logger,
	}
}

// SendBanAlert 发送 IP 封禁告警邮件
//
// 参数:
// - ip: 被封禁的 IP 地址
// - region: IP 归属地
//
// 返回值:
// - error: 发送邮件的错误
func (e *EmailAlert) SendBanAlert(ip, region string) error {
	if len(e.To) == 0 {
		e.logger.Warn("邮件告警未配置接收人")
		return nil
	}

	body := "sshwarden 告警\n"
	body += "======================\n"
	body += "IP: " + ip + "\n"
	body += "归属地: " + region + "\n"
	body += "状态: 已被封禁\n"
	body += "======================\n"

	return e.sendEmail(e.Subject, body)
}

// SendUnbanAlert 发送 IP 解除封禁告警邮件
//
// 参数:
// - ip: 被解除封禁的 IP 地址
// - region: IP 归属地
//
// 返回值:
// - error: 发送邮件的错误
func (e *EmailAlert) SendUnbanAlert(ip, region string) error {
	if len(e.To) == 0 {
		e.logger.Warn("邮件告警未配置接收人")
		return nil
	}

	body := "sshwarden 告警\n"
	body += "======================\n"
	body += "IP: " + ip + "\n"
	body += "归属地: " + region + "\n"
	body += "状态: 已解除封禁\n"
	body += "======================\n"

	return e.sendEmail(e.Subject, body)
}

// sendEmail 发送邮件的内部方法
//
// 参数:
// - subject: 邮件主题
// - body: 邮件正文
//
// 返回值:
// - error: 发送邮件的错误
func (e *EmailAlert) sendEmail(subject, body string) error {
	auth := smtp.PlainAuth("", e.Username, e.Password, e.SMTPHost)

	message := strings.Builder{}
	message.WriteString("From: " + e.From + "\r\n")
	message.WriteString("To: " + strings.Join(e.To, ", ") + "\r\n")
	message.WriteString("Subject: " + subject + "\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	err := smtp.SendMail(e.SMTPHost+":"+e.SMTPPort, auth, e.From, e.To, []byte(message.String()))
	if err != nil {
		e.logger.Error("发送邮件告警失败", zap.Error(err))
		return err
	}

	e.logger.Info("邮件告警发送成功", zap.String("subject", subject))
	return nil
}
