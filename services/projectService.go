package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"io"
	"iotServer/models"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

type ProjectService struct{}

// Detail 根据项目ID查询项目详情
func (s *ProjectService) Detail(projectId int64) (*models.Project, error) {
	o := orm.NewOrm()
	project := &models.Project{Id: projectId}

	err := o.Read(project)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	return project, nil
}

// Create 创建项目
func (s *ProjectService) Create(userId int64, name, description, address, people, mobile, personNum, area string) (int64, error) {
	o := orm.NewOrm()

	// 创建项目对象
	project := &models.Project{
		Name:        name,
		Description: description,
		Address:     address,
		People:      people,
		Mobile:      mobile,
		PersonNum:   personNum,
		Area:        area,
		// 设置默认值
		Enable:    true,
		Images:    "image_default.png",
		Logo:      "logo_default.png",
		Icon:      "icon_default.png",
		IndexType: "/home",
		Ranges:    "-1d",
		User:      &models.User{Id: userId},
	}

	// 设置创建时间
	project.ActiveTime = time.Now()

	// 插入数据库
	id, err := o.Insert(project)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Edit 编辑项目信息
func (s *ProjectService) Edit(projectId int64, name, desc, addr, people, mobile,
	personNum, area *string, enable *bool, timeStr, indexType *string) error {

	o := orm.NewOrm()
	project := &models.Project{Id: projectId}

	err := o.Read(project)
	if err != nil {
		return fmt.Errorf("project not found")
	}

	// 更新字段
	if name != nil && *name != "" {
		project.Name = *name
	}
	if desc != nil && *desc != "" {
		project.Description = *desc
	}
	if addr != nil && *addr != "" {
		project.Address = *addr
	}
	if people != nil && *people != "" {
		project.People = *people
	}
	if mobile != nil && *mobile != "" {
		project.Mobile = *mobile
	}
	if personNum != nil && *personNum != "" {
		project.PersonNum = *personNum
	}
	if area != nil && *area != "" {
		project.Area = *area
	}
	if enable != nil {
		project.Enable = *enable
	}
	if timeStr != nil && *timeStr != "" {
		// 解析时间字符串，这里假设格式为 "2006-01-02"
		activeTime, err := time.Parse("2006-01-02", *timeStr)
		if err == nil {
			project.ActiveTime = activeTime
		}
	}
	if indexType != nil && *indexType != "" {
		project.IndexType = *indexType
	}

	_, err = o.Update(project)
	return err
}

// UploadImage 上传项目图片
func (s *ProjectService) UploadImage(projectId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	project := &models.Project{Id: projectId}

	err := o.Read(project)
	if err != nil {
		return "", fmt.Errorf("this project does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	ext := filepath.Ext(fileHeader.Filename)
	imageName := fmt.Sprintf("image_%d%s%s", projectId, s.randomString(11), ext)

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
	project.Images = imageURL
	_, err = o.Update(project)
	if err != nil {
		return "", fmt.Errorf("failed to update project")
	}

	return imageURL, nil
}

// UploadLogo 上传项目logo
func (s *ProjectService) UploadLogo(projectId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	project := &models.Project{Id: projectId}

	err := o.Read(project)
	if err != nil {
		return "", fmt.Errorf("this project does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	imageName := fmt.Sprintf("logo_%d%s.png", projectId, s.randomString(11))

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
	project.Logo = imageName
	_, err = o.Update(project)
	if err != nil {
		return "", fmt.Errorf("failed to update project")
	}

	return logoURL, nil
}

// UploadIcon 上传项目图标
func (s *ProjectService) UploadIcon(projectId int64, fileHeader multipart.FileHeader) (string, error) {
	o := orm.NewOrm()
	project := &models.Project{Id: projectId}

	err := o.Read(project)
	if err != nil {
		return "", fmt.Errorf("this project does not exist")
	}

	// 创建图片存放目录
	imagePath := "./static/dist"
	err = os.MkdirAll(imagePath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create image directory")
	}

	// 生成图片名称
	imageName := fmt.Sprintf("icon_%d%s.png", projectId, s.randomString(11))

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
	project.Icon = iconURL
	_, err = o.Update(project)
	if err != nil {
		return "", fmt.Errorf("failed to update project")
	}

	return iconURL, nil
}

// randomString 生成随机字符串
func (s *ProjectService) randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
