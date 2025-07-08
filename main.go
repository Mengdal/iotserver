package main

import (
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
	"github.com/beego/beego/v2/server/web/filter/cors"
	"github.com/kardianos/service"
	"iotServer/common"
	"iotServer/controllers"
	_ "iotServer/routers"
	"iotServer/utils"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 清理 go clean -modcache
// go mod tidy
// 启动swagger bee run -gendoc=true -downdoc=true
// 手动执行 bee generate docs以及bee generate routers重新生成commentsRouter_controllers.go，新版本删除了自动生成功能
func initSwagger() {
	if beego.BConfig.RunMode == "dev" {
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.BConfig.MaxMemory = 1048576 // 文件上传默认内存缓存大小
	// 允许跨域
	beego.InsertFilter("*", beego.BeforeRouter, cors.Allow(&cors.Options{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Access-Control-Allow-Origin", "Content-Type", "token"},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
	}))
	//设置全局拦截
	beego.InsertFilter("/*", beego.BeforeRouter, func(ctx *context.Context) {
		if ctx.Input.Method() == "OPTIONS" {
			return
		}
		if !validateToken(ctx) {
			return
		}
	})
}
func validateToken(ctx *context.Context) bool {
	// 跳过 Swagger、静态文件等路径
	skipPaths := []string{
		"/swagger",
		"/static",
		"/favicon.ico",
		"/health",
		"/task/receive",
		"/api/user/login",
		"/api/user/register",
		"/api/scada/aggQueryHistory",
		"/api/scada/booQueryHistory",
		"/api/scada/diffQueryHistory",
		"/api/scada/getTagsTree",
		"/api/scada/queryHistory",
		"/api/scada/queryReal",
		"/api/ws",
	}
	for _, path := range skipPaths {
		if strings.HasPrefix(ctx.Request.URL.Path, path) {
			return true
		}
	}

	token := ctx.Request.Header.Get("Authorization")
	if token == "" {
		ctx.Output.SetStatus(401)
		ctx.Output.JSON(map[string]string{"error": "Unauthorized: missing token"}, false, false)
		return false
	}

	claims, ok := utils.ParseToken(token)
	if !ok {
		ctx.Output.SetStatus(403)
		ctx.Output.JSON(map[string]string{"Forbidden": "invalid token"}, false, false)
		return false
	}

	// 将 user_id 存入上下文供后续方法使用
	if userId, ok := claims["user_id"].(float64); ok {
		ctx.Input.SetData("user_id", int64(userId))
	}

	return true
}

func main() {
	if beego.BConfig.RunMode == "dev" {
		runDev()
	} else {
		common.SetCommandParam() //接收命令行参数
		ServiceOperate(common.Service())
	}
}

// 开发模式
func runDev() {
	common.InitDB()
	go controllers.InitMQTT()
	initSwagger()
	beego.Run()
}

type program struct {
	exitCh chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.exitCh = make(chan struct{})
	go p.run()
	return nil
}
func (p *program) run() {
	//设置当前工作目录
	setWorkingDirectoryToExecPath()
	// 初始化日志（必须放在这里，确保在 goroutine 中生效）
	logFile, err := os.OpenFile(`service.log`, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("无法打开日志文件:", err)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("【Service】服务启动中")

	// 延迟一小段时间确保服务注册完成
	time.Sleep(500 * time.Millisecond)

	log.Println("【Service】初始化数据库...")
	common.InitDB()

	log.Println("【Service】初始化Web服务...")
	initSwagger()

	log.Println("【Service】启动 Web 服务...")
	beego.Run()

	log.Println("【Service】服务已启动...")

	<-p.exitCh // 阻塞直到收到 Stop 信号
}

func setWorkingDirectoryToExecPath() {
	exePath, err := os.Executable()
	if err != nil {
		panic("无法获取可执行文件路径: " + err.Error())
	}
	exeDir := filepath.Dir(exePath)
	err = os.Chdir(exeDir)
	if err != nil {
		panic("无法切换工作目录: " + err.Error())
	}
}
func (p *program) Stop(s service.Service) error {
	close(p.exitCh)
	return nil
}
func ServiceOperate(ServiceType string) {
	srvConfig := &service.Config{
		Name:        "iotServer",
		DisplayName: "LM IoT Server Service",
		Description: "提供LM物联网管理的后端服务",
	}
	prg := &program{}
	s, err := service.New(prg, srvConfig)
	if err != nil {
		fmt.Println(err)
	}

	if ServiceType == "install" {
		err := s.Install()
		if err != nil {
			fmt.Println("安装服务失败: ", err.Error())
		} else {
			fmt.Println("安装服务成功")
			err := s.Start()
			if err != nil {
				fmt.Println("运行服务失败: ", err.Error())
			}
		}
		return
	} else if ServiceType == "uninstall" {
		err := s.Uninstall()
		if err != nil {
			fmt.Println("卸载服务失败: ", err.Error())
		} else {
			fmt.Println("卸载服务成功")
			err := s.Stop()
			if err != nil {
				fmt.Println("停止服务失败: ", err.Error())
			}
		}
		return
	}
	err = s.Run()
	if err != nil {
		fmt.Println(err)
	}
}
