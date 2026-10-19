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
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
	jwt2 "iotserver/internal/tools/jwt"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultUserName = "admin"
	DefaultLang     = "zh-CN"
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
	// 平台作用域：系统路径，不按租户过滤
	cfg, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).SysConfigByKey(key)
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
	// 平台作用域：系统路径，不按租户过滤
	return uapp.dbClient.WithCtx(tenant.Platform(context.Background())).MigrateSystemTables()
}

// UserLogin 用户登录
//
// 登录发生在租户上下文建立之前，因此这里显式用**平台作用域**查用户，
// 查到后再按用户的 tenant_id 组装 claims —— 这是唯一允许跨租户查用户的路径。
func (uapp *userApp) UserLogin(ctx context.Context, req dtos.LoginRequest) (res dtos.LoginResponse, err error) {
	platformCtx := tenant.Platform(ctx)
	platformDb := uapp.dbClient.WithCtx(platformCtx)

	user, edgeXErr := platformDb.GetUserByUserName(req.Username)
	if edgeXErr != nil {
		uapp.saveLoginLog(ctx, "", req.Username, models.LoginStatusFail, "用户不存在")
		return res, errort.NewCommonEdgeX(errort.AppPasswordError, "", edgeXErr)
	}

	if cErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); cErr != nil {
		uapp.saveLoginLog(ctx, user.TenantId, req.Username, models.LoginStatusFail, "密码错误")
		return res, errort.NewCommonErr(errort.AppPasswordError, cErr)
	}

	if user.Status == 0 {
		uapp.saveLoginLog(ctx, user.TenantId, req.Username, models.LoginStatusFail, "账号已停用")
		return res, errort.NewCommonErr(errort.AppAccountDisabled, fmt.Errorf("account %s is disabled", req.Username))
	}

	// 到期校验：平台管理员（tenant_id=''）不受影响；租户查不到时不阻碍登录，仅告警
	if user.TenantId != "" {
		t, tErr := platformDb.TenantById(user.TenantId)
		if tErr != nil {
			if uapp.lc != nil {
				uapp.lc.Warnf("登录到期校验: 租户 %s 查询失败，按不阻碍处理: %v", user.TenantId, tErr)
			}
		} else if t.ExpireAt > 0 && time.Now().UnixMilli() > t.ExpireAt {
			uapp.saveLoginLog(ctx, user.TenantId, req.Username, models.LoginStatusFail, "租户已到期")
			return res, errort.NewCommonErr(errort.AppTenantExpired,
				fmt.Errorf("tenant %s expired at %d", t.Id, t.ExpireAt))
		}
	}

	lang := user.Lang
	if lang == "" {
		lang = DefaultLang
	}

	loginUser, pErr := uapp.buildLoginUser(platformCtx, user, lang)
	if pErr != nil {
		return res, pErr
	}

	j := jwt2.NewJWT(jwt2.JwtSignKey)
	claims := middleware.CustomClaims{
		ID:            parseClaimUserId(user.Id),
		Username:      req.Username,
		TenantId:      user.TenantId,
		DeptId:        user.DeptId,
		DataScope:     loginUser.DataScope,
		PlatformAdmin: loginUser.PlatformAdmin,
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

	_ = platformDb.UpdateSysUserLastLogin(user.Id, time.Now().UnixMilli())
	uapp.saveLoginLog(ctx, user.TenantId, user.Username, models.LoginStatusSuccess, "登录成功")

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

// parseClaimUserId 把用户的字符串ID写进 claims（历史字段，解析失败时退化为 0）
func parseClaimUserId(id string) uint {
	v, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0
	}
	return uint(v)
}

// buildLoginUser 组装登录用户信息（角色/权限/菜单/租户）
func (uapp *userApp) buildLoginUser(ctx context.Context, user models.User, lang string) (dtos.LoginUserResponse, error) {
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
		TenantId:    user.TenantId,
	}

	// 角色/权限查询按传入 ctx 的作用域执行：登录时是平台作用域，个人中心时是用户所属租户
	db := uapp.dbClient.WithCtx(ctx)

	roleIds, err := db.GetUserRoleIds(user.Id)
	if err != nil {
		return result, err
	}
	result.RoleIds = roleIds

	roles, err := db.SysRolesByIds(roleIds)
	if err != nil {
		return result, err
	}

	admin := false
	scope := ""
	for _, r := range roles {
		result.Roles = append(result.Roles, r.RoleKey)
		if r.RoleKey == seed.RoleAdminKey {
			admin = true
		}
		scope = widestDataScope(scope, r.DataScope)
	}
	result.Admin = admin
	result.DataScope = scope
	// 平台管理员 = 内置超级管理员 且 不属于任何租户（平台运营账号）
	result.PlatformAdmin = admin && user.TenantId == ""

	assigned, err := uapp.menusOfUser(user.Id, user.TenantId, admin)
	if err != nil {
		return result, err
	}
	// 平台级菜单（如「租户管理」）只下发给平台管理员：租户用户即使被授权也看不到
	if !result.PlatformAdmin {
		assigned = filterPlatformOnlyMenus(assigned)
	}
	// 权限只按角色实际授权的菜单计算：补全父目录与展示层覆盖（隐藏/改名）都不影响权限集合
	result.Permissions = collectPerms(assigned)
	treeMenus := assigned
	if !admin {
		// 角色授权时可能只勾了子菜单没勾父目录，补全后菜单树才是连通的
		if treeMenus, err = uapp.completeMenuAncestors(assigned, user.TenantId); err != nil {
			return result, err
		}
	}
	// 租户展示层覆盖：隐藏（含其子树）/自定义名称/图标/排序
	if treeMenus, err = uapp.applyMenuPrefs(user.TenantId, treeMenus); err != nil {
		return result, err
	}
	result.Menus = buildMenuTree(treeMenus)

	if user.TenantId != "" {
		if t, tErr := db.TenantById(user.TenantId); tErr == nil {
			result.TenantName = t.Name
		}
	}
	if user.DeptId != "" {
		if dept, dErr := db.SysDeptById(user.DeptId); dErr == nil {
			result.DeptName = dept.Name
		}
	}
	return result, nil
}

// filterPlatformOnlyMenus 过滤平台级菜单（platform_only=1），租户用户不可见
func filterPlatformOnlyMenus(menus []models.Menu) []models.Menu {
	out := make([]models.Menu, 0, len(menus))
	for _, m := range menus {
		if m.PlatformOnly == 1 {
			continue
		}
		out = append(out, m)
	}
	return out
}

// widestDataScope 取多个角色里最宽的数据范围：全部 > 本部门及以下 > 本部门 > 仅本人
func widestDataScope(current, candidate string) string {
	rank := func(scope string) int {
		switch scope {
		case models.DataScopeAll:
			return 4
		case models.DataScopeDeptChild:
			return 3
		case models.DataScopeDept:
			return 2
		case models.DataScopeSelf:
			return 1
		default:
			return 0
		}
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}

// menusOfUser 查询用户可见菜单（角色授权 + 菜单作用域合并），不做展示层覆盖。
//
// 菜单是「全局默认 + 租户自建」表，不是行级隔离表，作用域在这里显式拼接：
//   - 平台用户（tenantId=""）：只看平台菜单 tenant_id=”；
//   - 租户用户：平台菜单 + 本租户自建菜单 tenant_id IN (”, tenantId)。
func (uapp *userApp) menusOfUser(userId, tenantId string, admin bool) ([]models.Menu, error) {
	// 平台作用域：菜单作用域由 tenantId 参数显式表达，不能依赖 GORM 插件
	db := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	if admin {
		return db.SysMenuAll(tenantId)
	}
	return db.RoleMenusByUserId(userId, tenantId)
}

/******************************* 当前登录用户 *******************************/

// IsAdminUsername 判断用户名是否为**平台**超级管理员。
//
// 重要：租户内的同名 admin 角色不算平台管理员 —— 平台管理员必须不属于任何租户
// （user.tenant_id 为空）。否则租户管理员会拿到「权限直通」，从而访问平台级接口
// （如租户管理），造成越权。
func (uapp *userApp) IsAdminUsername(username string) bool {
	if username == "" {
		return false
	}
	// 系统级查询：按全局唯一的用户名定位用户，因此显式使用平台作用域
	sysDb := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	user, err := sysDb.GetUserByUserName(username)
	if err != nil {
		return false
	}
	if user.TenantId != "" {
		return false
	}
	return uapp.hasAdminRole(sysDb, user.Id)
}

// PermissionsByUsername 获取用户权限标识集合。
//
// 与 buildLoginUser 保持一致：
//   - 拥有内置 admin 角色的用户在该租户内拥有全部租户级权限（租户管理员）；
//   - 平台级菜单（platform_only=1，如租户管理）只对平台管理员生效，租户用户一律过滤掉。
func (uapp *userApp) PermissionsByUsername(username string) ([]string, error) {
	sysDb := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	user, err := sysDb.GetUserByUserName(username)
	if err != nil {
		return nil, err
	}
	menus, err := uapp.menusOfUser(user.Id, user.TenantId, uapp.hasAdminRole(sysDb, user.Id))
	if err != nil {
		return nil, err
	}
	if user.TenantId != "" {
		menus = filterPlatformOnlyMenus(menus)
	}
	return collectPerms(menus), nil
}

// hasAdminRole 判断用户是否拥有内置 admin 角色（租户内的 admin 同样算，代表该租户的管理员）
func (uapp *userApp) hasAdminRole(db interfaces.DBClient, userId string) bool {
	adminRole, _ := uapp.roleInfo(db, userId)
	return adminRole
}

// roleInfo 汇总用户的角色信息：是否内置 admin 角色 + 最宽数据范围
func (uapp *userApp) roleInfo(db interfaces.DBClient, userId string) (adminRole bool, scope string) {
	roleIds, err := db.GetUserRoleIds(userId)
	if err != nil {
		return false, ""
	}
	roles, err := db.SysRolesByIds(roleIds)
	if err != nil {
		return false, ""
	}
	for _, r := range roles {
		if r.RoleKey == seed.RoleAdminKey {
			adminRole = true
		}
		scope = widestDataScope(scope, r.DataScope)
	}
	return adminRole, scope
}

// CurrentUserProfile 个人中心
func (uapp *userApp) CurrentUserProfile(ctx context.Context, username string) (dtos.SysUserProfileResponse, error) {
	var res dtos.SysUserProfileResponse
	// 按当前登录态的作用域查用户（同租户内），避免跨租户读取他人资料
	user, err := uapp.dbClient.WithCtx(ctx).GetUserByUserName(username)
	if err != nil {
		return res, err
	}
	loginUser, err := uapp.buildLoginUser(ctx, user, user.Lang)
	if err != nil {
		return res, err
	}
	res.User = uapp.toSysUserResponse(user)
	res.User.RoleIds = loginUser.RoleIds
	// RoleNames 用角色名称（toSysUserResponse 已按 roleIds 查好），不要覆盖成角色编码——
	// 个人中心是展示用的，返回 "admin" 这种编码用户看不懂。
	res.User.Admin = loginUser.Admin
	res.Permissions = loginUser.Permissions
	res.Roles = res.User.RoleNames
	res.Admin = loginUser.Admin
	return res, nil
}

// UpdateCurrentUserLang 个人中心-切换语言：存到账号上，换设备登录也能保持一致。
func (uapp *userApp) UpdateCurrentUserLang(ctx context.Context, username string, req dtos.UpdateLangRequest) error {
	lang := i18n.NormalizeLang(req.Lang)
	if lang == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("不支持的语言: {{.p1}}", map[string]interface{}{"p1": req.Lang}))
	}
	user, err := uapp.dbClient.WithCtx(ctx).GetUserByUserName(username)
	if err != nil {
		return err
	}
	user.Lang = lang
	user.Modified = nowMilli()
	return uapp.dbClient.WithCtx(ctx).UpdateUser(user)
}

// UpdateCurrentUserProfile 个人中心-修改资料
func (uapp *userApp) UpdateCurrentUserProfile(ctx context.Context, username string, req dtos.SysUserProfileUpdateRequest) error {
	user, err := uapp.dbClient.WithCtx(ctx).GetUserByUserName(username)
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
	return uapp.dbClient.WithCtx(ctx).UpdateUser(user)
}

/******************************* 登录相关 *******************************/

// InitInfo 查询用户信息
func (uapp *userApp) InitInfo() (res dtos.InitInfoResponse, err error) {
	// 平台作用域：系统路径，不按租户过滤
	_, edgeXErr := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).GetUserByUserName(DefaultUserName)
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

	_, edgeXErr := uapp.dbClient.WithCtx(ctx).GetUserByUserName(DefaultUserName)
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
	_, edgeXErr = uapp.dbClient.WithCtx(ctx).AddUser(newUser)
	if edgeXErr != nil {
		lc.Errorf("add user error %v", edgeXErr)
		return edgeXErr
	}
	// 绑定超级管理员角色
	if _, err = uapp.dbClient.WithCtx(ctx).SysRoleById(seed.RoleAdminId); err == nil {
		_ = uapp.dbClient.WithCtx(ctx).SetUserRoles(newUser.Id, []string{seed.RoleAdminId})
	}
	return nil
}

// UpdateUserPassword 修改密码
func (uapp *userApp) UpdateUserPassword(ctx context.Context, username string, req dtos.UpdatePasswordRequest) error {
	user, edgeXErr := uapp.dbClient.WithCtx(ctx).GetUserByUserName(username)
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
	return uapp.dbClient.WithCtx(ctx).UpdateUser(user)
}

// OpenApiUserLogin openapi用户登录
func (uapp *userApp) OpenApiUserLogin(ctx context.Context, req dtos.LoginRequest) (res *dtos.TokenDetail, err error) {
	// OpenAPI 登录同样发生在租户上下文建立之前：平台作用域查用户
	user, edgeXErr := uapp.dbClient.WithCtx(tenant.Platform(ctx)).GetUserByUserName(req.Username)
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

// CreateTokenDetail 根据用户名生成 token（OpenAPI 用）
//
// token 必须带上租户/部门/数据范围：OpenAPI 请求同样走租户隔离，
// 否则 enforce 策略下开放接口只能看到平台级数据。
func (uapp *userApp) CreateTokenDetail(userName string) (*dtos.TokenDetail, error) {
	td := &dtos.TokenDetail{
		AccessId:  "accessId",
		RefreshId: "refreshId",
		AtExpires: time.Now().Add(time.Minute * 120).Unix(),   //两小时
		RtExpires: time.Now().Add(time.Hour * 24 * 14).Unix(), //两星期
	}
	// 平台作用域查用户（OpenAPI 登录/刷新发生在租户上下文建立之前）
	sysDb := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	var (
		userID        uint
		tenantId      string
		deptId        string
		dataScope     string
		platformAdmin bool
	)
	if user, err := sysDb.GetUserByUserName(userName); err == nil {
		userID = parseClaimUserId(user.Id)
		tenantId = user.TenantId
		deptId = user.DeptId
		adminRole, scope := uapp.roleInfo(sysDb, user.Id)
		dataScope = scope
		platformAdmin = adminRole && user.TenantId == ""
	}
	base := middleware.CustomClaims{
		ID:            userID,
		Username:      userName,
		TenantId:      tenantId,
		DeptId:        deptId,
		DataScope:     dataScope,
		PlatformAdmin: platformAdmin,
	}
	var err error
	td.AccessToken, err = uapp.createToken(base, td.AtExpires, jwt2.JwtSignKey)
	if err != nil {
		return nil, err
	}
	td.RefreshToken, err = uapp.createToken(base, td.RtExpires, jwt2.RefreshKey)
	if err != nil {
		return nil, err
	}
	return td, nil
}

// createToken 生成 token（StandardClaims 由本方法统一填充）
func (uapp *userApp) createToken(claims middleware.CustomClaims, expire int64, signKey string) (string, error) {
	claims.StandardClaims = jwt.StandardClaims{
		NotBefore: time.Now().Unix(), // 签名生效时间
		ExpiresAt: expire,
		Issuer:    jwt2.JwtIssuer, // 签名的发行者
	}
	j := jwt2.NewJWT(signKey)
	token, jwtErr := j.CreateToken(claims)
	if jwtErr != nil {
		err := jwtErr
		return "", err
	}
	return token, nil
}

func (uapp *userApp) InitJwtKey() {
	// 平台作用域：系统路径，不按租户过滤
	user, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).GetUserByUserName(DefaultUserName)
	if err != nil {
		return
	}
	if user.GatewayKey == "" {
		user.GatewayKey = jwt2.GenerateJwtSignKey()
	}
	if user.OpenAPIKey == "" {
		user.OpenAPIKey = jwt2.GenerateJwtSignKey()
	}
	if err = uapp.dbClient.WithCtx(tenant.Platform(context.Background())).UpdateUser(user); err != nil {
		panic(err)
	}
	jwt2.SetOpenAPIKey(user.OpenAPIKey)
	jwt2.SetJwtSignKey(user.GatewayKey)
}

/******************************* 内部工具 *******************************/

// saveLoginLog 记录登录审查日志（type=login，module=认证，action=登录）
func (uapp *userApp) saveLoginLog(ctx context.Context, tenantId, username string, status int, message string) {
	ip, browser, osName := "", "", ""
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx != nil {
		ip = ginCtx.ClientIP()
		ua := ginCtx.Request.UserAgent()
		browser = utils.GetBrowser(ua)
		osName = utils.GetOs(ua)
	}
	now := time.Now().UnixMilli()
	log := models.AuditLog{
		Id:        utils.RandomNum(),
		Type:      models.AuditTypeLogin,
		Module:    models.AuditModuleAuth,
		Action:    models.AuditActionLogin,
		Username:  username,
		Ip:        ip,
		Browser:   browser,
		Os:        osName,
		Status:    status,
		Message:   message,
		LoginTime: now,
	}
	// 提升字段（内嵌 Timestamps）不能在复合字面量里赋值，构造后单独设置
	log.Created = now
	log.Modified = now
	// 审查日志按用户所属租户落库：租户内可见，账号不存在时记到平台级
	scopeCtx := tenant.Platform(ctx)
	if tenantId != "" {
		scopeCtx = tenant.With(ctx, tenant.Context{TenantID: tenantId})
	}
	if err := uapp.dbClient.WithCtx(scopeCtx).AddSysAuditLog(log); err != nil && uapp.lc != nil {
		uapp.lc.Errorf("save audit log error %v", err)
	}
}
