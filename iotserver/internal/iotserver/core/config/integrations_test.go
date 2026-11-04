package config

import (
	"testing"

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

[Integrations.Iotp]
Enabled = true
BaseURL = 'http://host:8080'
Token = 'tok'
Database = 'iotserver'
TimeoutSec = 5

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
	if !cfg.Integrations.Iotp.Enabled || cfg.Integrations.Iotp.BaseURL != "http://host:8080" ||
		cfg.Integrations.Iotp.Database != "iotserver" || cfg.Integrations.Iotp.TimeoutSec != 5 {
		t.Fatalf("Iotp 映射错误: %+v", cfg.Integrations.Iotp)
	}
	if !cfg.Integrations.CollectEngine.Enabled || cfg.Integrations.CollectEngine.TimeoutSec != 8 {
		t.Fatalf("CollectEngine 映射错误: %+v", cfg.Integrations.CollectEngine)
	}
}
