package userapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/utils"
	jwt2 "iotserver/internal/tools/jwt"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultUserName = "admin"
	DefaultLang     = "en"
)

var _ interfaces.UserItf = new(userApp)

type userApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

func New(dic *di.Container) *userApp {
	return &userApp{
		dic:      dic,
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		dbClient: container.DBClientFrom(dic.Get),
	}
}

// configInt 读取整型系统配置，缺失或非法时返回默认值
func (uapp *userApp) configInt(key string, fallback int) int {
	cfg, err := uapp.dbClient.SysConfigByKey(key)
	if err != nil || strings.TrimSpace(cfg.ConfigValue) == "" {
		return fallback
	}
	v, convErr := strconv.Atoi(strings.TrimSpace(cfg.ConfigValue))
	if convErr != nil || v <= 0 {
		return fallback
	}
	return v
}

// passwordMinLength 密码最小长度（读取「安全配置」项）
func (uapp *userApp) passwordMinLength() int {
	return uapp.configInt(seed.ConfigKeyPasswordMinLength, 6)
}

// sessionTimeoutMinutes 会话超时时间（分钟），默认 3 天
func (uapp *userApp) sessionTimeoutMinutes() int {
	return uapp.configInt(seed.ConfigKeySessionTimeout, 60*24*3)
}

// validatePassword 校验密码是否满足最小长度要求
func (uapp *userApp) validatePassword(password string) error {
	minLen := uapp.passwordMinLength()
	if len(password) < minLen {
		// 把要求长度放进 message，供 i18n 模板 {{.info}} 渲染
		return errort.NewCommonErr(errort.AppPasswordTooShort,
			fmt.Errorf("%d", minLen))
	}
	return nil
}

// MigrateSystemData 建表与初始化系统管理数据
func (uapp *userApp) MigrateSystemData() error {
	return uapp.dbClient.MigrateSystemTables()
}

// UserLogin 用户登录
func (uapp *userApp) UserLogin(ctx context.Context, req dtos.LoginRequest) (res dtos.LoginResponse, err error) {
	user, edgeXErr := uapp.dbClient.GetUserByUserName(req.Username)
	if edgeXErr != nil {
		uapp.saveLoginLog(ctx, req.Username, models.LoginStatusFail, "用户不存在")
		return res, errort.NewCommonEdgeX(errort.AppPasswordError, "", edgeXErr)
	}

	if cErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); cErr != nil {
		uapp.saveLoginLog(ctx, req.Username, models.LoginStatusFail, "密码错误")
		return res, errort.NewCommonErr(errort.AppPasswordError, cErr)
	}

	if user.Status == 0 {
		uapp.saveLoginLog(ctx, req.Username, models.LoginStatusFail, "账号已停用")
		return res, errort.NewCommonErr(errort.AppAccountDisabled, fmt.Errorf("account %s is disabled", req.Username))
	}

	j := jwt2.NewJWT(jwt2.JwtSignKey)
	claims := middleware.CustomClaims{
		ID:       1,
		Username: req.Username,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix() - 1000,                                   // 签名生效时间
			ExpiresAt: time.Now().Unix() + int64(uapp.sessionTimeoutMinutes())*60, // 过期时间读「安全配置」
			Issuer:    jwt2.JwtIssuer,                                             // 签名的发行者
		},
	}

	token, jwtErr := j.CreateToken(claims)
	if jwtErr != nil {
		return res, jwtErr
	}

	lang := user.Lang
	if lang == "" {
		lang = DefaultLang
	}

	loginUser, pErr := uapp.buildLoginUser(user, lang)
	if pErr != nil {
		return res, pErr
	}

	_ = uapp.dbClient.UpdateSysUserLastLogin(user.Id, time.Now().UnixMilli())
	uapp.saveLoginLog(ctx, user.Username, models.LoginStatusSuccess, "登录成功")

	res = dtos.LoginResponse{
		User:        loginUser,
		Token:       token,
		ExpiresAt:   claims.StandardClaims.ExpiresAt * 1000,
		IsInit:      true,
		Roles:       loginUser.Roles,
		Permissions: loginUser.Permissions,
		Menus:       loginUser.Menus,
	}
	return
}

// buildLoginUser 组装登录用户信息（角色/权限/菜单）
func (uapp *userApp) buildLoginUser(user models.User, lang string) (dtos.LoginUserResponse, error) {
	result := dtos.LoginUserResponse{
		Id:          user.Id,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Email:       user.Email,
		Phone:       user.Phone,
		DeptId:      user.DeptId,
		Lang:        lang,
		Roles:       []string{},
		RoleIds:     []string{},
		Permissions: []string{},
		Menus:       []dtos.SysMenuResponse{},
	}

	roleIds, err := uapp.dbClient.GetUserRoleIds(user.Id)
	if err != nil {
		return result, err
	}
	result.RoleIds = roleIds

	roles, err := uapp.dbClient.SysRolesByIds(roleIds)
	if err != nil {
		return result, err
	}

	admin := false
	for _, r := range roles {
		result.Roles = append(result.Roles, r.RoleKey)
		if r.RoleKey == seed.RoleAdminKey {
			admin = true
		}
	}
	result.Admin = admin

	assigned, err := uapp.menusOfUser(user.Id, admin)
	if err != nil {
		return result, err
	}
	// 权限只按角色实际授权的菜单计算，补全父目录不影响权限集合
	result.Permissions = collectPerms(assigned)
	treeMenus := assigned
	if !admin {
		// 角色授权时可能只勾了子菜单没勾父目录，补全后菜单树才是连通的
		if treeMenus, err = uapp.completeMenuAncestors(assigned); err != nil {
			return result, err
		}
	}
	result.Menus = buildMenuTree(treeMenus)

	if user.DeptId != "" {
		if dept, dErr := uapp.dbClient.SysDeptById(user.DeptId); dErr == nil {
			result.DeptName = dept.Name
		}
	}
	return result, nil
}

// menusOfUser 查询用户可见菜单，管理员返回全部菜单
func (uapp *userApp) menusOfUser(userId string, admin bool) ([]models.Menu, error) {
	if admin {
		return uapp.dbClient.SysMenuAll()
	}
	return uapp.dbClient.RoleMenusByUserId(userId)
}

/******************************* 当前登录用户 *******************************/

// IsAdminUsername 判断用户名是否为超级管理员
func (uapp *userApp) IsAdminUsername(username string) bool {
	if username == "" {
		return false
	}
	user, err := uapp.dbClient.GetUserByUserName(username)
	if err != nil {
		return false
	}
	roleIds, err := uapp.dbClient.GetUserRoleIds(user.Id)
	if err != nil {
		return false
	}
	roles, err := uapp.dbClient.SysRolesByIds(roleIds)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if r.RoleKey == seed.RoleAdminKey {
			return true
		}
	}
	return false
}

// PermissionsByUsername 获取用户权限标识集合
func (uapp *userApp) PermissionsByUsername(username string) ([]string, error) {
	user, err := uapp.dbClient.GetUserByUserName(username)
	if err != nil {
		return nil, err
	}
	admin := uapp.IsAdminUsername(username)
	menus, err := uapp.menusOfUser(user.Id, admin)
	if err != nil {
		return nil, err
	}
	return collectPerms(menus), nil
}

// CurrentUserProfile 个人中心
func (uapp *userApp) CurrentUserProfile(ctx context.Context, username string) (dtos.SysUserProfileResponse, error) {
	var res dtos.SysUserProfileResponse
	user, err := uapp.dbClient.GetUserByUserName(username)
	if err != nil {
		return res, err
	}
	loginUser, err := uapp.buildLoginUser(user, user.Lang)
	if err != nil {
		return res, err
	}
	res.User = uapp.toSysUserResponse(user)
	res.User.RoleIds = loginUser.RoleIds
	res.User.RoleNames = loginUser.Roles
	res.User.Admin = loginUser.Admin
	res.Permissions = loginUser.Permissions
	res.Roles = loginUser.Roles
	res.Admin = loginUser.Admin
	return res, nil
}

// UpdateCurrentUserProfile 个人中心-修改资料
func (uapp *userApp) UpdateCurrentUserProfile(ctx context.Context, username string, req dtos.SysUserProfileUpdateRequest) error {
	user, err := uapp.dbClient.GetUserByUserName(username)
	if err != nil {
		return err
	}
	user.Nickname = req.Nickname
	user.Email = req.Email
	user.Phone = req.Phone
	if req.Lang != "" {
		user.Lang = req.Lang
	}
	user.Modified = nowMilli()
	return uapp.dbClient.UpdateUser(user)
}

/******************************* 登录相关 *******************************/

// InitInfo 查询用户信息
func (uapp *userApp) InitInfo() (res dtos.InitInfoResponse, err error) {
	_, edgeXErr := uapp.dbClient.GetUserByUserName(DefaultUserName)
	if edgeXErr != nil {
		if errort.Is(errort.DefaultResourcesNotFound, edgeXErr) {
			res.IsInit = false
			return
		}
		return
	}
	res.IsInit = true
	return
}

// InitPassword 初始化密码
func (uapp *userApp) InitPassword(ctx context.Context, req dtos.InitPasswordRequest) error {
	lc := uapp.lc

	_, edgeXErr := uapp.dbClient.GetUserByUserName(DefaultUserName)
	if edgeXErr == nil {
		return errort.NewCommonErr(errort.AppSystemInitialized, edgeXErr)
	}

	if pErr := uapp.validatePassword(req.NewPassword); pErr != nil {
		return pErr
	}
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	newUser := models.User{
		Id:         utils.GenUUID(),
		Username:   DefaultUserName,
		Password:   string(newPasswordHash),
		Nickname:   "超级管理员",
		Status:     1,
		DeptId:     seed.DeptRootId,
		Lang:       DefaultLang,
		OpenAPIKey: jwt2.GenerateJwtSignKey(),
		GatewayKey: jwt2.GenerateJwtSignKey(),
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	jwt2.SetOpenAPIKey(newUser.OpenAPIKey)
	jwt2.SetJwtSignKey(newUser.GatewayKey)
	_, edgeXErr = uapp.dbClient.AddUser(newUser)
	if edgeXErr != nil {
		lc.Errorf("add user error %v", edgeXErr)
		return edgeXErr
	}
	// 绑定超级管理员角色
	if _, err = uapp.dbClient.SysRoleById(seed.RoleAdminId); err == nil {
		_ = uapp.dbClient.SetUserRoles(newUser.Id, []string{seed.RoleAdminId})
	}
	return nil
}

// UpdateUserPassword 修改密码
func (uapp *userApp) UpdateUserPassword(ctx context.Context, username string, req dtos.UpdatePasswordRequest) error {
	user, edgeXErr := uapp.dbClient.GetUserByUserName(username)
	if edgeXErr != nil {
		return edgeXErr
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword))
	if err != nil {
		return errort.NewCommonErr(errort.AppPasswordError, err)
	}

	if pErr := uapp.validatePassword(req.NewPassword); pErr != nil {
		return pErr
	}
	newPasswordHash, gErr := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if gErr != nil {
		return gErr
	}
	user.Password = string(newPasswordHash)
	user.Modified = nowMilli()
	return uapp.dbClient.UpdateUser(user)
}

// OpenApiUserLogin openapi用户登录
func (uapp *userApp) OpenApiUserLogin(ctx context.Context, req dtos.LoginRequest) (res *dtos.TokenDetail, err error) {
	user, edgeXErr := uapp.dbClient.GetUserByUserName(req.Username)
	if edgeXErr != nil {
		return res, errort.NewCommonErr(errort.AppPasswordError, edgeXErr)
	}

	cErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if cErr != nil {
		err = errort.NewCommonErr(errort.AppPasswordError, cErr)
		return
	}
	if user.Status == 0 {
		return res, errort.NewCommonErr(errort.AppAccountDisabled, fmt.Errorf("account %s is disabled", req.Username))
	}
	td, err := uapp.CreateTokenDetail(req.Username)
	if err != nil {
		return
	}
	return td, nil
}

// CreateTokenDetail 根据用户名生成 token
func (uapp *userApp) CreateTokenDetail(userName string) (*dtos.TokenDetail, error) {
	td := &dtos.TokenDetail{
		AccessId:  "accessId",
		RefreshId: "refreshId",
		AtExpires: time.Now().Add(time.Minute * 120).Unix(),   //两小时
		RtExpires: time.Now().Add(time.Hour * 24 * 14).Unix(), //两星期
	}
	var (
		userID uint = 1
		err    error
	)
	td.AccessToken, err = uapp.createToken(userID, userName, td.AtExpires, jwt2.JwtSignKey)
	if err != nil {
		return nil, err
	}
	td.RefreshToken, err = uapp.createToken(userID, userName, td.RtExpires, jwt2.RefreshKey)
	if err != nil {
		return nil, err
	}
	return td, nil
}

// createToken 生成 token
func (uapp *userApp) createToken(useId uint, userName string, expire int64, signKey string) (string, error) {
	j := jwt2.NewJWT(signKey)
	claims := middleware.CustomClaims{
		ID:       useId,
		Username: userName,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix(), // 签名生效时间
			ExpiresAt: expire,
			Issuer:    jwt2.JwtIssuer, // 签名的发行者
		},
	}

	token, jwtErr := j.CreateToken(claims)
	if jwtErr != nil {
		err := jwtErr
		return "", err
	}
	return token, nil
}

func (uapp *userApp) InitJwtKey() {
	user, err := uapp.dbClient.GetUserByUserName(DefaultUserName)
	if err != nil {
		return
	}
	if user.GatewayKey == "" {
		user.GatewayKey = jwt2.GenerateJwtSignKey()
	}
	if user.OpenAPIKey == "" {
		user.OpenAPIKey = jwt2.GenerateJwtSignKey()
	}
	if err = uapp.dbClient.UpdateUser(user); err != nil {
		panic(err)
	}
	jwt2.SetOpenAPIKey(user.OpenAPIKey)
	jwt2.SetJwtSignKey(user.GatewayKey)
}

/******************************* 内部工具 *******************************/

// saveLoginLog 记录登入日志
func (uapp *userApp) saveLoginLog(ctx context.Context, username string, status int, message string) {
	ip, browser, osName := "", "", ""
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		ip = ginCtx.ClientIP()
		ua := ginCtx.Request.UserAgent()
		browser = utils.GetBrowser(ua)
		osName = utils.GetOs(ua)
	}
	log := models.LoginLog{
		Id:        utils.RandomNum(),
		Username:  username,
		Ip:        ip,
		Browser:   browser,
		Os:        osName,
		Status:    status,
		Message:   message,
		LoginTime: time.Now().UnixMilli(),
	}
	if err := uapp.dbClient.AddSysLoginLog(log); err != nil && uapp.lc != nil {
		uapp.lc.Errorf("save login log error %v", err)
	}
}
