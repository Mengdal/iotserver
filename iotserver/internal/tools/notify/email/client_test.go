package email

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestParseRecipients(t *testing.T) {
	got, err := ParseRecipients("ops@example.com, oncall@example.com; ops@example.com")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0] != "ops@example.com" || got[1] != "oncall@example.com" {
		t.Fatalf("unexpected recipients: %#v", got)
	}
	if _, err := ParseRecipients("not-an-email"); err == nil {
		t.Fatal("expected invalid recipient error")
	}
}

func TestBuildMessage(t *testing.T) {
	raw := string(BuildMessage("alerts@example.com", []string{"ops@example.com"}, "设备告警：冷库 A", "库温超过 8℃\n请及时处理"))
	if !strings.HasPrefix(raw, "From: alerts@example.com\r\n") {
		t.Fatalf("missing From header: %q", raw)
	}
	if !strings.Contains(raw, "To: ops@example.com\r\n") {
		t.Fatalf("missing To header: %q", raw)
	}
	if !strings.Contains(raw, "Subject: =?utf-8?") {
		t.Fatalf("subject should be encoded: %q", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("missing content type: %q", raw)
	}
	if !strings.Contains(raw, "库温超过 8℃\r\n请及时处理") {
		t.Fatalf("body newlines should be CRLF: %q", raw)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("empty config should fail")
	}
	cfg := Config{Host: "smtp.example.com", Port: 587, From: "alerts@example.com"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
}

// fakeSMTPServer 只实现测试所需的最少 SMTP 命令，用于验证完整的发送链路。
func fakeSMTPServer(t *testing.T) (host string, port int, messages <-chan string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan string, 4)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		w := func(s string) { _, _ = fmt.Fprintf(conn, "%s\r\n", s) }
		w("220 localhost ESMTP")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(line)
			upper := strings.ToUpper(cmd)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				w("250-localhost")
				w("250 8BITMIME")
			case strings.HasPrefix(upper, "MAIL FROM"):
				w("250 OK")
			case strings.HasPrefix(upper, "RCPT TO"):
				w("250 OK")
			case upper == "DATA":
				w("354 End data with <CR><LF>.<CR><LF>")
				var b strings.Builder
				for {
					dataLine, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.TrimSpace(dataLine) == "." {
						break
					}
					b.WriteString(dataLine)
				}
				ch <- b.String()
				w("250 OK")
			case upper == "QUIT":
				w("221 Bye")
				return
			default:
				w("250 OK")
			}
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, ch, func() { _ = ln.Close() }
}

func TestSendOverSMTP(t *testing.T) {
	host, port, messages, stop := fakeSMTPServer(t)
	defer stop()

	cfg := Config{Host: host, Port: port, From: "alerts@example.com", Timeout: 5 * time.Second}
	if err := Send(cfg, "ops@example.com", "设备告警", "库温超过 8℃"); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case raw := <-messages:
		if !strings.Contains(raw, "Subject:") || !strings.Contains(raw, "库温超过 8℃") {
			t.Fatalf("unexpected message: %q", raw)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("smtp server did not receive message")
	}
}
