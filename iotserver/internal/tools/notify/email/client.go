// Package email 提供告警邮件发送能力（标准 SMTP）。
package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config SMTP 配置。
//
// TLS 为 true 时：465 端口走隐式 TLS，其它端口在服务器支持时升级 STARTTLS。
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLS      bool
	Timeout  time.Duration
}

// Validate 检查发送邮件所需的最小配置。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("smtp host is not configured")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid smtp port: %d", c.Port)
	}
	if strings.TrimSpace(c.From) == "" && strings.TrimSpace(c.Username) == "" {
		return errors.New("smtp from address is not configured")
	}
	if _, err := mail.ParseAddress(c.fromAddress()); err != nil {
		return fmt.Errorf("invalid smtp from address: %w", err)
	}
	return nil
}

func (c Config) fromAddress() string {
	if strings.TrimSpace(c.From) != "" {
		return strings.TrimSpace(c.From)
	}
	return strings.TrimSpace(c.Username)
}

// Send 发送纯文本邮件。recipients 支持逗号、分号或空白分隔。
func Send(cfg Config, recipients string, subject, body string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	to, err := ParseRecipients(recipients)
	if err != nil {
		return err
	}
	if len(to) == 0 {
		return errors.New("email recipient is empty")
	}
	raw := BuildMessage(cfg.fromAddress(), to, subject, body)
	return sendRaw(cfg, to, raw)
}

// ParseRecipients 解析收件人列表并校验地址格式。
func ParseRecipients(raw string) ([]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	seen := make(map[string]bool, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		addr, err := mail.ParseAddress(field)
		if err != nil {
			return nil, fmt.Errorf("invalid email recipient %q: %w", field, err)
		}
		key := strings.ToLower(addr.Address)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, addr.Address)
	}
	return out, nil
}

// BuildMessage 生成 RFC 5322 纯文本邮件。导出便于单测。
func BuildMessage(from string, to []string, subject, body string) []byte {
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	b.WriteString("\r\n")
	return []byte(b.String())
}

func sendRaw(cfg Config, to []string, raw []byte) error {
	addr := net.JoinHostPort(strings.TrimSpace(cfg.Host), strconv.Itoa(cfg.Port))
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	if cfg.TLS && cfg.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: cfg.Host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if cfg.TLS && cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if strings.TrimSpace(cfg.Username) != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(envelopeAddress(cfg.fromAddress())); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

// envelopeAddress 去掉 "Name <addr>" 里的显示名，SMTP 信封只接受地址。
func envelopeAddress(raw string) string {
	if addr, err := mail.ParseAddress(raw); err == nil {
		return addr.Address
	}
	return raw
}
