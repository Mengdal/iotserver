# IoT Server

基于 Beego 框架构建的轻量级物联网平台，提供设备管理、数据采集、规则引擎、告警处理等核心功能。

## 📁 项目结构

<pre>
iotserver/
├── common/         # 第三方组件和公共工具
├── conf/           # 配置文件（端口、数据库等）
├── controllers/    # 控制器层（API 接口实现）
├── database/       # SQLite 数据库文件
├── iotp/           # IOT EdgeDB 服务层
├── models/         # 数据模型和 DTO 定义
├── mount/          # Docker 挂载目录
├── routers/        # 路由配置
├── scada/          # 组态插件
├── services/       # 业务逻辑服务层
├── static/         # 前端静态资源
├── swagger/        # API 文档
├── utils/          # 工具类函数
└── service.xx      # 系统服务文件
</pre>

## 🛠️ 开发指南

### 环境要求
- Go 1.18+
- Bee 工具（可选，用于开发）

### 安装依赖
<pre>
go mod tidy
</pre>

### 运行项目
<pre>
bee run
</pre>

### 生成文档
<pre>
bee run -gendoc=true -downdoc=true
</pre>

### 代码生成
<pre>
bee generate docs     # 生成 Swagger 文档
bee generate routers  # 生成路由文件
go build -o iotserver.exe # 生成可执行文件
</pre>

## 📚 API 文档

访问 <code>http://localhost:8088/swagger/</code> 查看完整的 API 文档。

## 🐳 项目部署

<pre>
右键service_install.bat管理员启动安装，卸载使用右键service_uninstall.bat文件
</pre>


## 📖 技术栈

- **后端框架**: [Beego v2](https://beego.vip/)
- **数据库**: SQLite3
- **ORM**: Beego ORM
- **文档**: Swagger UI
- **MQTT**: Eclipse Paho
- **流处理**: LF Edge eKuiper
