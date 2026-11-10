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
`
	var cfg ConfigurationStruct
	if _, err := toml.Decode(raw, &cfg); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.Integrations.EdgeIngest.Broker != "tcp://host:1885" ||
		cfg.Integrations.EdgeIngest.ResyncIntervalSec != 30 {
		t.Fatalf("EdgeIngest 映射错误: %+v", cfg.Integrations.EdgeIngest)
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

func TestDeploymentMode(t *testing.T) {
	var cfg ConfigurationStruct
	if cfg.DeploymentMode() != DeploymentModeServer {
		t.Fatalf("empty mode should default to server, got %q", cfg.DeploymentMode())
	}
	cfg.Deployment.Mode = "EMBEDDED"
	if cfg.DeploymentMode() != DeploymentModeEmbedded {
		t.Fatalf("embedded mode should be normalized, got %q", cfg.DeploymentMode())
	}
	cfg.Deployment.Mode = "unknown"
	if cfg.DeploymentMode() != DeploymentModeServer {
		t.Fatalf("unknown mode should fall back to server, got %q", cfg.DeploymentMode())
	}
}

func TestPluginDirectories(t *testing.T) {
	var cfg ConfigurationStruct
	if cfg.PluginRoot() != "data/plugins" || cfg.PluginDataDir() != "data/plugin-data" {
		t.Fatalf("unexpected default plugin dirs: %q %q", cfg.PluginRoot(), cfg.PluginDataDir())
	}
	cfg.Plugins.Root = "/opt/plugins"
	cfg.Plugins.DataDir = "/var/lib/plugin-data"
	if cfg.PluginRoot() != "/opt/plugins" || cfg.PluginDataDir() != "/var/lib/plugin-data" {
		t.Fatalf("unexpected configured plugin dirs: %q %q", cfg.PluginRoot(), cfg.PluginDataDir())
	}
}

func TestWebBuildPath(t *testing.T) {
	var cfg ConfigurationStruct
	if cfg.WebBuildPath() != "./cmd/iotserver-ui/build" {
		t.Fatalf("unexpected default web build path: %q", cfg.WebBuildPath())
	}
	cfg.WebUI.BuildPath = "/opt/iotserver/ui"
	if cfg.WebBuildPath() != "/opt/iotserver/ui" {
		t.Fatalf("unexpected configured web build path: %q", cfg.WebBuildPath())
	}
}
