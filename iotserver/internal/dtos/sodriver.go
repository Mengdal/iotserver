package dtos

import (
	"iotserver/internal/models"
)

// SODriverUploadRequest 上传 iotpGW .so 驱动时的元数据。
type SODriverUploadRequest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
	Arch        string `json:"arch"`
	ABI         string `json:"abi"`
	Description string `json:"description"`
	FileName    string `json:"fileName"`
	SHA256      string `json:"sha256"`
}

// SODriverDTO 已登记/已安装的 iotpGW .so 驱动。
type SODriverDTO struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
	Arch        string `json:"arch"`
	ABI         string `json:"abi"`
	FileName    string `json:"fileName"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	Updated     int64  `json:"updated"`
}

// SODriverDTOFromModel 把驱动库记录和安装状态转成接口 DTO。
func SODriverDTOFromModel(library models.DeviceLibrary, fileName, abi string, installed bool) SODriverDTO {
	return SODriverDTO{
		Id:          library.Id,
		Name:        library.Name,
		Version:     library.Version,
		Protocol:    library.Protocol,
		Arch:        library.Arch,
		ABI:         abi,
		FileName:    fileName,
		Description: library.Description,
		Installed:   installed,
		Size:        library.PackageSize,
		Hash:        library.PackageHash,
		Updated:     library.Modified,
	}
}
