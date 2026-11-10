package pluginpkg

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract 把插件包解压到 dest。拒绝绝对路径、.. 和符号链接，避免路径穿越。
func Extract(archivePath, dest string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("不是有效的 tar.gz: %w", err)
	}
	defer gz.Close()

	dest = filepath.Clean(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(header.Name, "./")
		if name == "" {
			continue
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("插件包不允许包含链接文件: %s", name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		cleanDest := dest + string(os.PathSeparator)
		if !strings.HasPrefix(target, cleanDest) {
			return fmt.Errorf("插件包包含非法路径: %s", name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, reader)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			// 忽略设备文件、FIFO 等特殊类型。
			continue
		}
	}
	return nil
}

// EntryPath 返回插件入口在解压目录中的绝对路径，并确保不越界。
func EntryPath(versionDir, entrypoint string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(entrypoint))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("非法的 entrypoint: %s", entrypoint)
	}
	target := filepath.Join(versionDir, clean)
	root := filepath.Clean(versionDir) + string(os.PathSeparator)
	if !strings.HasPrefix(target, root) {
		return "", fmt.Errorf("entrypoint 越界: %s", entrypoint)
	}
	return target, nil
}
