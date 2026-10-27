// Package twilio 通过 Twilio API 发送邮件、短信与 WhatsApp 通知。
//
// 三个渠道共用同一套 Account SID / Auth Token；邮件、短信、WhatsApp 的发送方
// 分别配置。WhatsApp 业务主动消息需要使用预先审核通过的 Content Template；
// 未配置模板时回退为 Body，仅适用于 Sandbox 或 24 小时会话窗口内。
package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL       = "https://api.twilio.com"
	defaultEmailBaseURL  = "https://comms.twilio.com"
	defaultEmailFromName = "iotserver"
)

// Config Twilio 配置。
type Config struct {
	AccountSID         string
	AuthToken          string
	EmailFrom          string
	SMSFrom            string
	WhatsAppFrom       string
	WhatsAppContentSID string
	// DefaultCountryCode 本地号码缺少国家码时使用，例如沙特 966。
	DefaultCountryCode string
	// BaseURL 仅用于测试；生产环境留空。
	BaseURL string
	// EmailBaseURL 仅用于测试；生产环境留空。
	EmailBaseURL string
	// HTTPClient 可选，默认 15 秒超时。
	HTTPClient *http.Client
}

// Validate 检查账号凭据。
func (c Config) Validate() error {
	if strings.TrimSpace(c.AccountSID) == "" {
		return errors.New("twilio account sid is not configured")
	}
	if strings.TrimSpace(c.AuthToken) == "" {
		return errors.New("twilio auth token is not configured")
	}
	return nil
}

// Client Twilio 消息客户端。
type Client struct {
	cfg Config
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// SendSMS 发送普通短信。to 会被规范化为 E.164。
func (c *Client) SendSMS(ctx context.Context, to, body string) error {
	if strings.TrimSpace(c.cfg.SMSFrom) == "" {
		return errors.New("twilio sms from is not configured")
	}
	return c.send(ctx, NormalizePhone(to, c.cfg.DefaultCountryCode), strings.TrimSpace(c.cfg.SMSFrom), body, "", nil)
}

// SendEmail 通过 Twilio Email API 发送纯文本邮件。
//
// recipients 支持逗号、分号或空白分隔；发件地址必须属于 Twilio 控制台里
// 已验证的 Sender Domain。该接口返回 202 表示已受理，最终送达需看 Twilio 状态。
func (c *Client) SendEmail(ctx context.Context, recipients, subject, body string) error {
	if strings.TrimSpace(c.cfg.EmailFrom) == "" {
		return errors.New("twilio email from is not configured")
	}
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	to, err := parseEmailRecipients(recipients)
	if err != nil {
		return err
	}
	if len(to) == 0 {
		return errors.New("email recipient is empty")
	}
	from, err := mail.ParseAddress(strings.TrimSpace(c.cfg.EmailFrom))
	if err != nil {
		return fmt.Errorf("invalid twilio email from: %w", err)
	}
	payload := map[string]interface{}{
		"from": map[string]string{"address": from.Address, "name": defaultEmailFromName},
		"to":   to,
		"content": map[string]string{
			"subject": subject,
			"text":    body,
			"html":    strings.ReplaceAll(html.EscapeString(body), "\n", "<br>\n"),
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode twilio email: %w", err)
	}
	base := strings.TrimRight(strings.TrimSpace(c.cfg.EmailBaseURL), "/")
	if base == "" {
		base = defaultEmailBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/Emails", strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.cfg.AccountSID, c.cfg.AuthToken)

	client := c.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio email request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio email http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func parseEmailRecipients(raw string) ([]map[string]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	seen := make(map[string]bool, len(fields))
	out := make([]map[string]string, 0, len(fields))
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
		out = append(out, map[string]string{"address": addr.Address})
	}
	return out, nil
}

// SendWhatsApp 发送 WhatsApp 消息。
//
// contentSID 非空时使用模板消息，variables 作为模板变量（key 从 1 开始）；
// 否则发送 body 文本。
func (c *Client) SendWhatsApp(ctx context.Context, to, body, contentSID string, variables map[string]string) error {
	if strings.TrimSpace(c.cfg.WhatsAppFrom) == "" {
		return errors.New("twilio whatsapp from is not configured")
	}
	if strings.TrimSpace(contentSID) == "" {
		contentSID = c.cfg.WhatsAppContentSID
	}
	to = WithWhatsAppPrefix(NormalizePhone(to, c.cfg.DefaultCountryCode))
	from := WithWhatsAppPrefix(strings.TrimSpace(c.cfg.WhatsAppFrom))
	return c.send(ctx, to, from, body, contentSID, variables)
}

func (c *Client) send(ctx context.Context, to, from, body, contentSID string, variables map[string]string) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(to) == "" {
		return errors.New("twilio recipient is empty")
	}
	form := url.Values{}
	form.Set("To", to)
	form.Set("From", from)
	if strings.TrimSpace(contentSID) != "" {
		form.Set("ContentSid", contentSID)
		if len(variables) > 0 {
			b, err := json.Marshal(variables)
			if err != nil {
				return fmt.Errorf("encode whatsapp variables: %w", err)
			}
			form.Set("ContentVariables", string(b))
		}
	} else {
		form.Set("Body", body)
	}

	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", base, url.PathEscape(c.cfg.AccountSID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.cfg.AccountSID, c.cfg.AuthToken)

	client := c.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var twErr struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(payload, &twErr) == nil && twErr.Message != "" {
			return fmt.Errorf("twilio error %d: %s", twErr.Code, twErr.Message)
		}
		return fmt.Errorf("twilio http %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

// WithWhatsAppPrefix 给号码补上 whatsapp: 前缀。
func WithWhatsAppPrefix(number string) string {
	number = strings.TrimSpace(number)
	if number == "" || strings.HasPrefix(strings.ToLower(number), "whatsapp:") {
		return number
	}
	return "whatsapp:" + number
}

// NormalizePhone 把常见输入规范成 E.164（保留 + 前缀）。
//
// 支持：
//   - +9665xxxxxxxx / 009665xxxxxxxx
//   - 本地号码 05xxxxxxxx（需提供默认国家码，如 966）
//   - 已经带国家码但缺少 + 的 9665xxxxxxxx
func NormalizePhone(raw, defaultCountryCode string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s)
	if strings.HasPrefix(s, "00") {
		return "+" + strings.TrimPrefix(s, "00")
	}
	if strings.HasPrefix(s, "+") {
		return s
	}
	cc := strings.TrimPrefix(strings.TrimSpace(defaultCountryCode), "+")
	if cc != "" {
		if strings.HasPrefix(s, "0") {
			return "+" + cc + s[1:]
		}
		if !strings.HasPrefix(s, cc) {
			return "+" + cc + s
		}
	}
	if strings.HasPrefix(s, "0") {
		return "+" + s[1:]
	}
	return "+" + s
}
