// Package pluginpkg 定义原生插件包格式与解压校验。
//
// 插件包是一个 tar.gz，根目录必须包含 manifest.json，entrypoint 相对包根目录。
package pluginpkg

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
)

const ManifestFile = "manifest.json"

// Manifest 插件清单。
type Manifest struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Name           string       `json:"name"`
	DisplayName    string       `json:"displayName"`
	Description    string       `json:"description"`
	Version        string       `json:"version"`
	Category       string       `json:"category"`
	Runtime        string       `json:"runtime"`
	Arch           []string     `json:"arch"`
	Protocols      []string     `json:"protocols"`
	Entrypoint     string       `json:"entrypoint"`
	Args           []string     `json:"args"`
	ConfigSchema   string       `json:"configSchema"`
	Health         *Health      `json:"health"`
	Ports          []Port       `json:"ports"`
	Permissions    []string     `json:"permissions"`
	Resources      Resource     `json:"resources"`
	Dependencies   []Dependency `json:"dependencies"`
	MinCoreVersion string       `json:"minCoreVersion"`
}

type Health struct {
	Type        string `json:"type"`
	Path        string `json:"path"`
	Port        string `json:"port"`
	IntervalSec int    `json:"intervalSec"`
	TimeoutSec  int    `json:"timeoutSec"`
}

type Port struct {
	Name    string `json:"name"`
	Default int    `json:"default"`
}

type Resource struct {
	MemoryMB   int `json:"memoryMB"`
	CPUPercent int `json:"cpuPercent"`
}

type Dependency struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
}

// Parse 解析并校验清单。
func Parse(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = 1
	}
	if manifest.Category == "" {
		manifest.Category = "driver"
	}
	if manifest.Runtime == "" {
		manifest.Runtime = "native"
	}
	if manifest.DisplayName == "" {
		manifest.DisplayName = manifest.Name
	}
	if !namePattern.MatchString(manifest.Name) {
		return manifest, fmt.Errorf("manifest.name 只能包含字母、数字、点、下划线和连字符")
	}
	if !versionPattern.MatchString(manifest.Version) {
		return manifest, fmt.Errorf("manifest.version 只能包含字母、数字、点、下划线、加号和连字符")
	}
	if strings.TrimSpace(manifest.Entrypoint) == "" {
		return manifest, fmt.Errorf("manifest.entrypoint 不能为空")
	}
	if len(manifest.Arch) == 0 {
		manifest.Arch = []string{"any"}
	}
	return manifest, nil
}

// MatchesArch 判断当前平台架构是否被清单支持。
func (m Manifest) MatchesArch() bool {
	target := runtime.GOARCH
	for _, arch := range m.Arch {
		arch = strings.ToLower(strings.TrimSpace(arch))
		if arch == "any" || arch == target {
			return true
		}
		if arch == "armv7" && target == "arm" {
			return true
		}
	}
	return false
}

// Load 从 tar.gz 中读取并解析 manifest.json。
func Load(archivePath string) (Manifest, error) {
	data, err := readArchiveFile(archivePath, ManifestFile)
	if err != nil {
		return Manifest{}, err
	}
	return Parse(data)
}

func readArchiveFile(archivePath, target string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("不是有效的 tar.gz: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(header.Name, "./")
		if name != target {
			continue
		}
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("插件包缺少 %s", target)
}
