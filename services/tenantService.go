package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"io"
	"iotServer/models"
	"iotServer/models/dtos"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

type TenantService struct{}

// Detail 根据租户ID查询租户详情
func (s *TenantService) Detail(tenantId, departmentId int64, page, size int) (*dtos.TenantDetailDTO, error) {
	o := orm.NewOrm()
	department := &models.Department{Id: departmentId}
	err := o.Read(department)
	if err != nil {
		return nil, fmt.Errorf("department not found")
	}

	tenant := &models.Tenant{PeerProjectId: departmentId}
	err = o.Read(tenant, "peer_project_id")
	detail := &dtos.TenantDetailDTO{
		// Department 字段
		Id:        department.Id,
		Name:      department.Name,
		LevelType: department.LevelType,
		TenantId:  department.TenantId,
		Leader:    department.Leader,
		Phone:     department.Phone,
		Email:     department.Email,
		Status:    department.Status,
		Sort:      department.Sort,
		Remark:    department.Remark,
		Created:   department.Created,
		Modified:  department.Modified,
		// 机构补充字段
		Factory:     department.Factory,
		Active:      department.Active,
		Description: department.Description,
		GIS:         department.GIS,
		Capacity:    department.Capacity,
		AreaId:      department.AreaId,
	}
	// 项目
	if err != nil && tenant.Id == 0 && department.LevelType == "PROJECT" {
		service := DepartmentService{}
		result, err := service.GetDirectDepartmentDevices(tenantId, departmentId, page, size)
		if err != nil {
			return nil, fmt.Errorf("department not found")
		}
		detail.Devices = result
	} else if department.LevelType == "DEPARTMENT" {
		// 对于部门级别的，单独设置部门地址
		detail.Address = department.Address
		return detail, nil
	} else { // 租户
		detail.Enable = tenant.Enable
		// 租户级别的地址
		detail.Area = tenant.Area
		detail.DeviceNum = tenant.DeviceNum
		detail.Images = tenant.Images
		detail.Logo = tenant.Logo
		detail.PersonNum = tenant.PersonNum
		detail.Ranges = tenant.Ranges
		detail.ActiveTime = tenant.ActiveTime
		detail.Icon = tenant.Icon
		detail.ExpirationTime = tenant.ExpirationTime
		// 租户级别的地址信息
		detail.Address = tenant.Address
	}

	return detail, nil
}

// Create 创建租户
func (s *TenantService) Create(userId, peerProjectId int64, name, address, personNum, area string) (int64, error) {
	o := orm.NewOrm()

	// 创建项目对象
	tenant := &models.Tenant{
		Name:      name,
		Address:   address,
		PersonNum: personNum,
		Area:      area,
		// 设置默认值
		Enable:        true,
		Images:        "image_default.png",
		Logo:          "logo_default.png",
		Icon:          "icon_default.png",
		IndexType:     "/home",
		Ranges:        "-1d",
		User:          &models.User{Id: userId},
		PeerProjectId: peerProjectId,
	}

	// 设置创建时间
	tenant.ActiveTime = time.Now()

	// 插入数据库
	id, err := o.Insert(tenant)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Edit 编辑项目信息
func (s *TenantService) Edit(departmentId int64, name, addr,
	personNum, area *string, enable *bool, timeStr, indexType *string) error {

	o := orm.NewOrm()
	tenant := &models.Tenant{PeerProjectId: departmentId}

	err := o.Read(tenant, "peer_project_id")
	if err != nil {
		return fmt.Errorf("tenant not found")
	}

	// 更新字段
	if name != nil && *name != "" {
		tenant.Name = *name
	}
	if addr != nil && *addr != "" {
		tenant.Address = *addr
	}
	if personNum != nil && *personNum != "" {
		tenant.PersonNum = *personNum
	}
	if area != nil && *area != "" {
		tenant.Area = *area
	}
	if enable != nil {
		tenant.Enable = *enable
	}
	if timeStr != nil && *timeStr != "" {
		// 解析时间字符串，这里假设格式为 "2006-01-02"
		activeTime, err := time.Parse("2006-01-02", *timeStr)
		if err == nil {
			tenant.ActiveTime = activeTime
		}
	}
	if indexType != nil && *indexType != "" {
		tenant.IndexType = *indexType
	}

	_, err = o.Update(tenant)
	return err
}

// UploadImage 上传项目图片
func (s *TenantService) UploadImage(departmentId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	tenant := &models.Tenant{PeerProjectId: departmentId}

	err := o.Read(tenant, "peer_project_id")
	if err != nil {
		return "", fmt.Errorf("this tenant does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist-pro"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	ext := filepath.Ext(fileHeader.Filename)
	imageName := fmt.Sprintf("image_%d%s%s", departmentId, s.randomString(11), ext)

	// 保存文件
	imageResultPath := filepath.Join(imagePath, imageName)

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file")
	}
	defer file.Close()

	dst, err := os.Create(imageResultPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file")
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", fmt.Errorf("failed to save file")
	}

	// 更新项目信息
	imageURL := imageName
	tenant.Images = imageURL
	_, err = o.Update(tenant)
	if err != nil {
		return "", fmt.Errorf("failed to update tenant")
	}

	return imageURL, nil
}

// UploadLogo 上传项目logo
func (s *TenantService) UploadLogo(departmentId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	tenant := &models.Tenant{PeerProjectId: departmentId}

	err := o.Read(tenant, "peer_project_id")
	if err != nil {
		return "", fmt.Errorf("this tenant does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist-pro"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	imageName := fmt.Sprintf("logo_%d%s.png", departmentId, s.randomString(11))

	// 保存文件
	imageResultPath := filepath.Join(imagePath, imageName)

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file")
	}
	defer file.Close()

	dst, err := os.Create(imageResultPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file")
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", fmt.Errorf("failed to save file")
	}

	// 更新项目信息
	logoURL := imageName
	tenant.Logo = imageName
	_, err = o.Update(tenant)
	if err != nil {
		return "", fmt.Errorf("failed to update tenant")
	}

	return logoURL, nil
}

// UploadIcon 上传项目图标
func (s *TenantService) UploadIcon(departmentId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	tenant := &models.Tenant{PeerProjectId: departmentId}

	err := o.Read(tenant, "peer_project_id")
	if err != nil {
		return "", fmt.Errorf("this tenant does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist-pro"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	imageName := fmt.Sprintf("icon_%d%s.png", departmentId, s.randomString(11))

	// 保存文件
	imageResultPath := filepath.Join(imagePath, imageName)

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file")
	}
	defer file.Close()

	dst, err := os.Create(imageResultPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file")
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", fmt.Errorf("failed to save file")
	}

	// 更新项目信息
	iconURL := imageName
	tenant.Icon = iconURL
	_, err = o.Update(tenant)
	if err != nil {
		return "", fmt.Errorf("failed to update tenant")
	}

	return iconURL, nil
}

// randomString 生成随机字符串
func (s *TenantService) randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
