package config

import (
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

// [Integrations.*] 下的外部接入配置必须能正确映射到 IntegrationsInfo。
func TestIntegrationsMapping(t *testing.T) {
	const raw = `
[Integrations.EdgeIngest]
Broker = 'tcp://host:1885'
Username = 'u'
Password = 'p'
ResyncIntervalSec = 30

[Integrations.CollectEngine]
Enabled = true
BaseURL = 'http://host:8081'
TimeoutSec = 8
`
	var cfg ConfigurationStruct
	if _, err := toml.Decode(raw, &cfg); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Integrations.EdgeIngest.Broker != "tcp://host:1885" ||
		cfg.Integrations.EdgeIngest.ResyncIntervalSec != 30 {
		t.Fatalf("EdgeIngest 映射错误: %+v", cfg.Integrations.EdgeIngest)
	}
	if !cfg.Integrations.CollectEngine.Enabled || cfg.Integrations.CollectEngine.TimeoutSec != 8 {
		t.Fatalf("CollectEngine 映射错误: %+v", cfg.Integrations.CollectEngine)
	}
}

// iotp 的连接参数归属在 [Databases.Data]，与 DuckDB 同一层。
func TestIotpDatabaseMapping(t *testing.T) {
	const raw = `
[Databases.Data]
Type = 'iotp'
BaseURL = 'http://host:8080'
Token = 'tok'
Name = 'iotserver'
Timeout = 5000
`
	var cfg ConfigurationStruct
	if _, err := toml.Decode(raw, &cfg); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	db, ok := cfg.IotpConfig()
	if !ok || db.Type != "iotp" || db.BaseURL != "http://host:8080" || db.Token != "tok" {
		t.Fatalf("iotp 映射错误: ok=%v db=%+v", ok, db)
	}
	if db.IotpDatabaseName() != "iotserver" || db.IotpTimeout() != 5*time.Second {
		t.Fatalf("iotp 缺省解析错误: name=%q timeout=%s", db.IotpDatabaseName(), db.IotpTimeout())
	}
}
