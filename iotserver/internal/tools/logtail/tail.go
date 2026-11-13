// Package logtail 读取日志文件末尾的若干行。
//
// 只用于平台配置里明确列出的组件日志文件，不接受客户端传入路径。
package logtail

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// DefaultLines 默认尾部行数。
	DefaultLines = 200
	// MaxLines 单次最多返回行数，避免把大文件整体读进内存。
	MaxLines = 1000
	// maxReadBytes 从文件末尾最多读取的字节数。
	maxReadBytes = 2 << 20
)

// ErrNotFound 日志文件不存在。
var ErrNotFound = errors.New("log file not found")

// Info 日志文件的基本状态。
type Info struct {
	Path     string
	Size     int64
	Modified int64
	Exists   bool
}

// Stat 返回日志文件状态；不存在时 Exists=false，不返回错误。
func Stat(path string) Info {
	path = strings.TrimSpace(path)
	info := Info{Path: path}
	if path == "" {
		return info
	}
	stat, err := os.Stat(path)
	if err != nil {
		return info
	}
	info.Exists = true
	info.Size = stat.Size()
	info.Modified = stat.ModTime().UnixMilli()
	return info
}

// Tail 返回文件末尾最多 maxLines 行。
func Tail(path string, maxLines int) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrNotFound
	}
	if maxLines <= 0 {
		maxLines = DefaultLines
	}
	if maxLines > MaxLines {
		maxLines = MaxLines
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log file: %w", err)
	}
	if stat.Size() == 0 {
		return []string{}, nil
	}

	readSize := stat.Size()
	if readSize > maxReadBytes {
		readSize = maxReadBytes
	}
	buf := make([]byte, readSize)
	if _, err = file.ReadAt(buf, stat.Size()-readSize); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read log file: %w", err)
	}
	text := string(buf)
	// 从文件中间开始读时，第一行可能是半行，丢掉。
	if stat.Size() > readSize {
		if index := strings.IndexByte(text, '\n'); index >= 0 {
			text = text[index+1:]
		}
	}
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return []string{}, nil
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
