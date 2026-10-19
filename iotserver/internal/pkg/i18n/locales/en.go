package locales

import "github.com/nicksnyder/go-i18n/v2/i18n"

func GetEnMessages() []*i18n.Message {
	return []*i18n.Message{
		{
			ID:    "10001",
			Other: `A system error has occurred while processing your request.`,
		},
		{
			ID:    "10002",
			Other: `Failed to parse JSON data.`,
		},
		{
			ID:    "10003",
			Other: `An error has occurred while processing your request parameters.{{.info | filter | left_space }}`,
		},
		{
			ID:    "10004",
			Other: `The requested resource does not exist.`,
		},
		{
			ID:    "10005",
			Other: `The file name cannot contain special characters.`,
		},
		{
			ID:    "10006",
			Other: `Request token is invalid.`,
		},
		{
			ID:    "10007",
			Other: `Request parameter name is duplicate`,
		},
		{
			ID:    "10008",
			Other: `Request parameter name cannot contain special characters`,
		},
		{
			ID:    "10009",
			Other: `Resource already exists, repeat operation`,
		},
		{
			ID:    "10010",
			Other: `The request ID is empty and the resource does not exist`,
		},
		{
			ID:    "10011",
			Other: `Upload file error`,
		},
		{
			ID:    "10012",
			Other: `Error reading Excel file`,
		},
		{
			ID:    "10013",
			Other: `Error reading Excel file, The required parameter is empty`,
		},
		{
			ID:    "10014",
			Other: `Database operation exception`,
		},
		{
			ID:    "10015",
			Other: `Unknown error`,
		},
		{
			ID:    "10016",
			Other: `MQTT service connection failed`,
		},

		{
			ID:    "20101",
			Other: `The specified password is incorrect.`,
		},
		{
			ID:    "20102",
			Other: `The system has been initialized. Please log in to the system.`,
		},
		{
			ID:    "20103",
			Other: `The account has been disabled. Please contact the administrator.`,
		},
		{
			ID:    "20104",
			Other: `The password must be at least {{.info}} characters long.`,
		},

		// System management
		{
			ID:    "21700",
			Other: `The user does not exist.`,
		},
		{
			ID:    "21701",
			Other: `The role does not exist.`,
		},
		{
			ID:    "21702",
			Other: `The role has been assigned and cannot be deleted.`,
		},
		{
			ID:    "21703",
			Other: `There are child departments. Delete them first.`,
		},
		{
			ID:    "21704",
			Other: `The department is in use by users and cannot be deleted.`,
		},
		{
			ID:    "21705",
			Other: `There are child menus. Delete them first.`,
		},
		{
			ID:    "21706",
			Other: `The parent cannot be itself or its own descendant.`,
		},
		{
			ID:    "21707",
			Other: `The built-in administrator data cannot be operated in this way.`,
		},
		{
			ID:    "21708",
			Other: `You do not have permission to access this resource.`,
		},
		{
			ID:    "21709",
			Other: `The config key already exists.`,
		},
		{
			ID:    "21710",
			Other: `The built-in solution is read-only (a tenant copy is created when a tenant modifies it).`,
		},
		{
			ID:    "21711",
			Other: `The tenant account has expired. Please contact the platform administrator.`,
		},
		{
			ID:    "21712",
			Other: `The resource quota has been reached: {{.info}}`,
		},
		{
			ID:    "21713",
			Other: `{{.info}}`,
		},
		{
			ID:    "21714",
			Other: `{{.info}}`,
		},
		{
			ID:    "22101",
			Other: `Connection refused. Make sure the cloud service is running.`,
		},

		// 驱动库
		{
			ID:    "20201",
			Other: `The driver library has been instantiated. Delete the driver instance first.`,
		},
		{
			ID:    "20204",
			Other: `The driver library is being upgraded, please try later.`,
		},
		{
			ID:    "20205",
			Other: `Docker warehouse permission verification failed, please check the permissions.`,
		},
		{
			ID:    "20206",
			Other: `The docker image version does not exist.`,
		},
		//三方服务
		{
			ID:    "20207",
			Other: `The app service has been instantiated. please Delete the app service instance first.`,
		},
		{
			ID:    "20211",
			Other: `Driver library not exist.`,
		},
		{
			ID:    "20212",
			Other: `Driver library failed to get default configuration.`,
		},
		{
			ID:    "20213",
			Other: `Driver image download failed.`,
		},
		{
			ID:    "20214",
			Other: `Driver image ID not found.`,
		},
		{
			ID:    "20215",
			Other: `The built-in driver does not allow deletion.`,
		},
		{
			ID:    "20216",
			Other: `The docker image repository does not exist.`,
		},
		{
			ID:    "20217",
			Other: `Time out.`,
		},

		// 驱动实例
		{
			ID:    "20301",
			Other: `The specified driver instance has been associated with devices. Delete the sub-devices first.`,
		},
		{
			ID:    "20302",
			Other: `The specified instance is running. Stop the instance first.`,
		},
		{
			ID:    "20303",
			Other: `The specified instance is running or stopping. Please wait.`,
		},
		{
			ID:    "20304",
			Other: `The specified instance setup is not yaml format`,
		},
		{
			ID:    "20305",
			Other: `The driver failed to issue commands`,
		},
		{
			ID:    "20306",
			Other: `The driver instance is not started, and the request is not responding`,
		},
		{
			ID:    "20307",
			Other: `The specified instance is running or stopping. Please wait.`,
		},
		{
			ID:    "20308",
			Other: `Failed to start the driver instance.`,
		},
		{
			ID:    "20309",
			Other: `The driver instance not exist.`,
		},
		{
			ID:    "20310",
			Other: `The driver instance stop fail.`,
		},
		{
			ID:    "20311",
			Other: `The driver instance docker params parse err.`,
		},
		{
			ID:    "20312",
			Other: `The driver instance container name repeat.`,
		},
		{
			ID:    "20313",
			Other: `Get available port fail.`,
		},
		{
			ID:    "20314",
			Other: `Failed to create config file.`,
		},
		{
			ID:    "20315",
			Other: `The driver instance not local platform.`,
		},

		// 设备

		{
			ID:    "20406",
			Other: `Product id not found when adding device.`,
		},
		{
			ID:    "20407",
			Other: `Thing model device deletion is not supported.`,
		},
		{
			ID:    "20408",
			Other: `Response of cloud API error when bindinging sub device`,
		},
		{
			ID:    "20409",
			Other: `Response of cloud API error when updating device attributes`,
		},
		{
			ID:    "20410",
			Other: `Device not exist`,
		},
		{
			ID:    "20411",
			Other: `Device command not exist`,
		},
		{
			ID:    "20412",
			Other: `Local devices are not allowed to connect to the platform`,
		},
		{
			ID:    "20413",
			Other: `Please unbind the device with the driver first`,
		},
		{
			ID:    "20414",
			Other: `The device platform is inconsistent with the drive platform`,
		},
		{
			ID:    "20415",
			Other: `This device has been bound to alarm rules. Please stop reporting relevant alarm rules before proceeding with the operation`,
		},
		{
			ID:    "20416",
			Other: `This device has been bound to scene rules. Please stop reporting scene rules before proceeding with the operation`,
		},

		// 产品
		{
			ID:    "20601",
			Other: `The specified product has been associated with DPs. Delete the DPs first.`,
		},
		{
			ID:    "20602",
			Other: `The specified product has been associated with sub-devices. Delete the sub-devices first.`,
		},
		{
			ID:    "20603",
			Other: `No active device found with the product, please add device and activate it.`,
		},
		{
			ID:    "20604",
			Other: `Product not exist.`,
		},

		{
			ID:    "20605",
			Other: `Sync failed.`,
		},

		{
			ID:    "20606",
			Other: `Product resource not exist.`,
		},

		{
			ID:    "20607",
			Other: `Please sync product data.`,
		},
		{
			ID:    "20608",
			Other: `Product property code exist.`,
		},
		{
			ID:    "20609",
			Other: `Product event code exist.`,
		},
		{
			ID:    "20610",
			Other: `Product server code exist.`,
		},
		{
			ID:    "20611",
			Other: `This product has been bound to alarm rules. Please stop reporting relevant alarm rules before proceeding with the operation.`,
		},
		{
			ID:    "20612",
			Other: `The product has not been released yet. Please release the product before adding devices`,
		},
		{
			ID:    "20613",
			Other: `Please cancel publishing the product first before proceeding with the operation`,
		},
		{
			ID:    "20614",
			Other: `Code identifier already exists`,
		},
		{
			ID:    "20616",
			Other: `The data type does not support modification. Please delete and recreate`,
		},

		// docker config
		{
			ID:    "20701",
			Other: `Please delete the bound driver library first`,
		},
		{
			ID:    "20702",
			Other: `Driver repository configuration does not exist`,
		},

		//专家系统
		{
			ID:    "20802",
			Other: `There is this scene in the timed task, please delete the timed task bound to this scene first`,
		},
		{
			ID:    "20803",
			Other: `There is content in the smart scene, please delete the content first`,
		},
		{
			ID:    "20831",
			Other: `There is content in the linkage strategy, please delete the content first`,
		},
		{
			ID:    "20832",
			Other: `There is content in the linkage strategy, please delete the content first`,
		},
		{
			ID:    "20833",
			Other: `Scheduled tasks that already exist`,
		},
		{
			ID:    "20860",
			Other: `The data type gateway is not yet supported`,
		},
		{
			ID:    "20861",
			Other: `Invalid job type`,
		},
		{
			ID:    "20862",
			Other: `Invalid action type`,
		},

		// 物模型
		{
			ID:    "20901",
			Other: "sync thing model failed",
		},
		{
			ID:    "20902",
			Other: "subDevice needed to be bond when sync thing model",
		},
		{
			ID:    "20903",
			Other: "parameters error or incomplete",
		},

		// 功能点
		{
			ID:    "21001",
			Other: `Sync function point，result return error.`,
		},
		{
			ID:    "21002",
			Other: `Sync function point，result return fail.`,
		},
		{
			ID:    "21003",
			Other: `Sync function point，parse fail.`,
		},
		{
			ID:    "21004",
			Other: `Function point not exist.`,
		},

		// ota
		{
			ID:    "21100",
			Other: "OTA upgrade error",
		},
		{
			ID:    "21101",
			Other: "Upgrading the gateway",
		},
		{
			ID:    "21102",
			Other: "OTA firmware download url error",
		},
		{
			ID:    "21103",
			Other: "OTA firmware download error",
		},
		{
			ID:    "21104",
			Other: "OTA verify firmware error",
		},
		{
			ID:    "21105",
			Other: "OTA execute upgrade command error",
		},
		{
			ID:    "21106",
			Other: "OTA upgrade firmware error",
		},
		{
			ID:    "21107",
			Other: "gateway does not support OTA",
		},
		{
			ID:    "21108",
			Other: "OTA get upgrade version error",
		},
		{
			ID:    "21109",
			Other: "OTA not upgrade version",
		},
		{
			ID:    "21110",
			Other: "OTA upgrade version wrong",
		},
		//category
		{
			ID:    "21200",
			Other: "The category does not exist.",
		},
		{
			ID:    "21201",
			Other: "The thingModel does not exist.",
		},

		//alert rule
		{
			ID:    "21300",
			Other: "Rule alert does not exist.",
		},
		{
			ID:    "21301",
			Other: "Invalid rule json.",
		},
		{
			ID:    "21302",
			Other: "Rule engine not found rule.",
		},
		{
			ID:    "21303",
			Other: "Rule is starting.",
		},
		{
			ID:    "21304",
			Other: "The product or device has been modified. Please edit the rule again.",
		},
		{
			ID:    "21305",
			Other: "Parameter error, please edit the rule again.",
		},
		{
			ID:    "21306",
			Other: "The format of the effective time is incorrect. The end time should be greater than the start time.",
		},
		{
			ID:    "21307",
			Other: "Please stop this rule before editing it.",
		},
		//scene
		{
			ID:    "21400",
			Other: "Please stop this scheduled tasks before editing it.",
		},
		{
			ID:    "21402",
			Other: "Parameter error, please edit the scene again.",
		},
		//rule
		{
			ID:    "21500",
			Other: "Please stop this rule engine before editing it.",
		},
		{
			ID:    "21600",
			Other: "Invalid resource configuration, please check the resource configuration.",
		},
		/***********************************************************/
		/*********************** string 翻译 ***********************/
		/**********************************************************/
		{
			ID:    "agentclient.alert.notify",
			Other: "notify",
		},
		{
			ID:    "agentclient.alert.warn",
			Other: "warn",
		},
		{
			ID:    "agentclient.alert.error",
			Other: "error",
		},
		{
			ID:    "agentclient.alert.system_alert",
			Other: "system_alert",
		},
		{
			ID:    "agentclient.alert.driver_alert",
			Other: "driver_alert",
		},

		{
			ID:    "default.success",
			Other: "Success",
		},
		{
			ID:    "default.fail",
			Other: "Fail",
		},
		{
			ID:    "library.upgrade_download",
			Other: "Driver Library {{.name}} Install/Upgrade {{.status}}",
		},
		{
			ID:    "service.run_status",
			Other: "Driver Service {{.name}} Run/Stop {{.status}}",
		},
		{
			ID:    "appService.run_status",
			Other: "service {{.name}} Install/Upgrade{{.status}}",
		},
		{
			ID:    "appServiceLibrary.upgrade_download",
			Other: "service instance {{.name}} Run/Stop{{.status}}",
		},
		{
			ID:    "appService.delete",
			Other: "service instance {{.name}} Delete{{.status}}",
		},
		{
			ID:    "cloudInstance.log_status",
			Other: "service instance {{.name}} log Run/Stop{{.status}}",
		},

		// 设备批量导入模板的列名
		{
			ID:    "device.import.Cid",
			Other: "Device ID",
		},
		{
			ID:    "device.import.Name",
			Other: "Device name",
		},
		{
			ID:    "device.import.Description",
			Other: "Description",
		},
		{
			ID:    "device.import.Ip",
			Other: "IP address",
		},
		{
			ID:    "device.import.Lat",
			Other: "Latitude",
		},
		{
			ID:    "device.import.Lon",
			Other: "Longitude",
		},
		{
			ID:    "device.import.VendorCode",
			Other: "Vendor",
		},
		{
			ID:    "device.import.InstallLocation",
			Other: "Installation location",
		},
		{
			ID:    "device.import.ExtendData",
			Other: "Extended fields",
		},
		{
			ID:    "device.import.IsIPC",
			Other: "Is IPC device [false/true]",
		},

		// 后端接口返回的细节文案（errort.NewMessageErr，ID 就是中文原文）
		{
			ID:    "boardId 不能为空",
			Other: "boardId is required",
		},
		{
			ID:    "name 不能为空",
			Other: "name is required",
		},
		{
			ID:    "series 格式有误",
			Other: "Invalid series format",
		},
		{
			ID:    "slot 只能是 home 或 default",
			Other: "slot must be home or default",
		},
		{
			ID:    "{{.p1}} 包含非法字符",
			Other: "{{.p1}} contains invalid characters",
		},
		{
			ID:    "{{.p1}} 长度不能超过 {{.p2}} 个字符",
			Other: "{{.p1}} must not exceed {{.p2}} characters",
		},
		{
			ID:    "{{.p1}}数量已达上限（当前 {{.p2}}，上限 {{.p3}}）",
			Other: "{{.p1}} limit reached (currently {{.p2}}, limit {{.p3}})",
		},
		{
			ID:    "一块看板最多 {{.p1}} 个组件",
			Other: "A board supports at most {{.p1}} widgets",
		},
		{
			ID:    "一次最多查询 {{.p1}} 个指标",
			Other: "At most {{.p1}} metrics per query",
		},
		{
			ID:    "一次最多查询 {{.p1}} 个组件",
			Other: "At most {{.p1}} widgets per query",
		},
		{
			ID:    "一次最多查询 {{.p1}} 条数据序列",
			Other: "At most {{.p1}} data series per query",
		},
		{
			ID:    "不支持的命令 {{.p1}}（可选 restart/ota/sync）",
			Other: "Unsupported command {{.p1}} (restart/ota/sync allowed)",
		},
		{
			ID:    "不支持的触发方式: {{.p1}}",
			Other: "Unsupported trigger: {{.p1}}",
		},
		{
			ID:    "不支持的触发类型: {{.p1}}",
			Other: "Unsupported trigger type: {{.p1}}",
		},
		{
			ID:    "不支持的资源类型: {{.p1}}",
			Other: "Unsupported resource type: {{.p1}}",
		},
		{
			ID:    "判断条件格式错误，应为 = 值（不含引号/分号）",
			Other: "Invalid condition format, expected = value (no quotes or semicolons)",
		},
		{
			ID:    "判断条件格式错误，应为 操作符+数值，例如 \"> 30\"",
			Other: "Invalid condition format, expected operator + number, e.g. \"> 30\"",
		},
		{
			ID:    "判断条件过长",
			Other: "Condition is too long",
		},
		{
			ID:    "参考系统接入 MQTT 连接失败: {{.p1}}",
			Other: "MQTT connection to the reference system failed: {{.p1}}",
		},
		{
			ID:    "只有平台管理员可以重置租户菜单定制",
			Other: "Only platform administrators can reset tenant menu customizations",
		},
		{
			ID:    "命令参数必须是合法 JSON",
			Other: "Command parameters must be valid JSON",
		},
		{
			ID:    "回调鉴权失败",
			Other: "Callback authentication failed",
		},
		{
			ID:    "场景未启用，忽略触发",
			Other: "The scene is not enabled, trigger ignored",
		},
		{
			ID:    "定时表达式不合法: {{.p1}}",
			Other: "Invalid schedule expression: {{.p1}}",
		},
		{
			ID:    "定时表达式无法匹配任何时间",
			Other: "The schedule expression never matches",
		},
		{
			ID:    "布尔型判断条件应为 = true 或 = false",
			Other: "Boolean conditions must be = true or = false",
		},
		{
			ID:    "平台原始菜单只有平台管理员可以维护",
			Other: "Only platform administrators can maintain platform menus",
		},
		{
			ID:    "当前数据层不支持在线导出",
			Other: "The current data layer does not support online export",
		},
		{
			ID:    "指标 {{.p1}} 不是时序指标",
			Other: "Metric {{.p1}} is not a time-series metric",
		},
		{
			ID:    "描述长度不能超过 {{.p1}} 个字符",
			Other: "Description must not exceed {{.p1}} characters",
		},
		{
			ID:    "数据库客户端为空",
			Other: "Database client is nil",
		},
		{
			ID:    "数据库连接为空",
			Other: "Database connection is nil",
		},
		{
			ID:    "数据报文解析失败: {{.p1}}",
			Other: "Failed to parse the data message: {{.p1}}",
		},
		{
			ID:    "数据资源 {{.p1}} 不存在或已被删除",
			Other: "Data resource {{.p1}} does not exist or has been deleted",
		},
		{
			ID:    "数据资源ID非法",
			Other: "Invalid data resource ID",
		},
		{
			ID:    "文本组件超过 {{.p1}} 字",
			Other: "Text widget exceeds {{.p1}} characters",
		},
		{
			ID:    "方案 {{.p1}} 不存在",
			Other: "Solution {{.p1}} does not exist",
		},
		{
			ID:    "方案 {{.p1}} 的物模型 JSON 序列化失败",
			Other: "Failed to serialize the thing model JSON of solution {{.p1}}",
		},
		{
			ID:    "方案 {{.p1}} 的物模型 JSON 解析失败",
			Other: "Failed to parse the thing model JSON of solution {{.p1}}",
		},
		{
			ID:    "方案 {{.p1}} 的物模型 {{.p2}} 解析失败",
			Other: "Failed to parse the thing model of solution {{.p1}}: {{.p2}}",
		},
		{
			ID:    "方案 {{.p1}} 的物模型保存失败",
			Other: "Failed to save the thing model of solution {{.p1}}",
		},
		{
			ID:    "方案 {{.p1}} 的租户副本创建失败",
			Other: "Failed to create the tenant copy of solution {{.p1}}",
		},
		{
			ID:    "最多绑定 {{.p1}} 条数据序列",
			Other: "At most {{.p1}} data series can be bound",
		},
		{
			ID:    "未支持的参考系统上行类型: {{.p1}}",
			Other: "Unsupported reference-system upstream type: {{.p1}}",
		},
		{
			ID:    "未注册的网关 pk={{.p1}} sn={{.p2}}",
			Other: "Unregistered gateway pk={{.p1}} sn={{.p2}}",
		},
		{
			ID:    "未注册的网关 sn={{.p1}}",
			Other: "Unregistered gateway sn={{.p1}}",
		},
		{
			ID:    "未登记的指标：{{.p1}}",
			Other: "Unregistered metric: {{.p1}}",
		},
		{
			ID:    "未知的上行类型: {{.p1}}",
			Other: "Unknown upstream type: {{.p1}}",
		},
		{
			ID:    "模板 key 不能为空",
			Other: "Template key is required",
		},
		{
			ID:    "父菜单 {{.p1}} 不在当前作用域内",
			Other: "Parent menu {{.p1}} is not in the current scope",
		},
		{
			ID:    "物模型条目 {{.p1}} 不存在",
			Other: "Thing model entry {{.p1}} does not exist",
		},
		{
			ID:    "物模型标识符 {{.p1}} 已存在，不能重复",
			Other: "Thing model identifier {{.p1}} already exists",
		},
		{
			ID:    "物模型标识符 {{.p1}} 已存在，不能重复添加",
			Other: "Thing model identifier {{.p1}} already exists and cannot be added again",
		},
		{
			ID:    "状态报文解析失败: {{.p1}}",
			Other: "Failed to parse the status message: {{.p1}}",
		},
		{
			ID:    "看板不存在：{{.p1}}",
			Other: "Board does not exist: {{.p1}}",
		},
		{
			ID:    "租户编码只允许小写字母/数字/下划线，长度 2~32",
			Other: "Tenant code allows only lowercase letters, digits and underscores, 2-32 characters",
		},
		{
			ID:    "租户自建菜单不允许设置权限码(perms)，平台菜单的权限由平台管理员维护",
			Other: "Tenant-created menus cannot set a permission identifier (perms); platform menu permissions are maintained by platform administrators",
		},
		{
			ID:    "第 {{.p1}} 个动作未选择功能",
			Other: "Action {{.p1}} has no function selected",
		},
		{
			ID:    "第 {{.p1}} 个动作未选择设备",
			Other: "Action {{.p1}} has no device selected",
		},
		{
			ID:    "第 {{.p1}} 个动作的{{.p2}}过长",
			Other: "{{.p2}} of action {{.p1}} is too long",
		},
		{
			ID:    "第 {{.p1}} 条数据序列缺少设备或属性",
			Other: "Data series {{.p1}} is missing a device or property",
		},
		{
			ID:    "组件 id 重复：{{.p1}}",
			Other: "Duplicate widget id: {{.p1}}",
		},
		{
			ID:    "组件「{{.p1}}」{{.p2}}",
			Other: "Widget \"{{.p1}}\": {{.p2}}",
		},
		{
			ID:    "组件「{{.p1}}」不能包含子组件",
			Other: "Widget \"{{.p1}}\" cannot contain child widgets",
		},
		{
			ID:    "组件嵌套最多 {{.p1}} 层",
			Other: "Widgets can be nested at most {{.p1}} levels deep",
		},
		{
			ID:    "统计 {{.p1}} 失败：{{.p2}}",
			Other: "Failed to aggregate {{.p1}}: {{.p2}}",
		},
		{
			ID:    "缺少 rule_id",
			Other: "Missing rule_id",
		},
		{
			ID:    "缺少定时表达式",
			Other: "Missing schedule expression",
		},
		{
			ID:    "缺少必填参数: {{.p1}}",
			Other: "Missing required parameter: {{.p1}}",
		},
		{
			ID:    "网关 {{.p1}} 上报的设备 {{.p2}} 未注册（productKey={{.p3}}）",
			Other: "Device {{.p2}} reported by gateway {{.p1}} is not registered (productKey={{.p3}})",
		},
		{
			ID:    "网关序列号不能为空",
			Other: "Gateway serial number is required",
		},
		{
			ID:    "网关接入 MQTT 连接失败: {{.p1}}",
			Other: "MQTT connection for gateway onboarding failed: {{.p1}}",
		},
		{
			ID:    "菜单 {{.p1}} 不属于当前作用域，不能授权",
			Other: "Menu {{.p1}} is not in the current scope and cannot be granted",
		},
		{
			ID:    "菜单 {{.p1}} 属于 {{.p2}} 作用域，当前作用域无权修改",
			Other: "Menu {{.p1}} belongs to the {{.p2}} scope; the current scope cannot modify it",
		},
		{
			ID:    "菜单定制属于租户视图，请先切换到租户后再操作",
			Other: "Menu customization applies to a tenant view; switch to a tenant first",
		},
		{
			ID:    "规则SQL过长",
			Other: "Rule SQL is too long",
		},
		{
			ID:    "规则描述长度不能超过 {{.p1}} 个字符",
			Other: "Rule description must not exceed {{.p1}} characters",
		},
		{
			ID:    "触发条件无法生成查询语句",
			Other: "The trigger conditions cannot produce a query statement",
		},
		{
			ID:    "触发条件缺少 {{.p1}}",
			Other: "Trigger conditions are missing {{.p1}}",
		},
		{
			ID:    "订阅失败: {{.p1}}",
			Other: "Subscribe failed: {{.p1}}",
		},
		{
			ID:    "记录缺少 deviceKey",
			Other: "Record is missing deviceKey",
		},
		{
			ID:    "设备属性设置异常: {{.p1}}",
			Other: "Failed to set device properties: {{.p1}}",
		},
		{
			ID:    "该租户下仍有 {{.p1}} 个用户、{{.p2}} 台设备、{{.p3}} 台网关、{{.p4}} 个产品、{{.p5}} 个看板，请先清理",
			Other: "This tenant still has {{.p1}} users, {{.p2}} devices, {{.p3}} gateways, {{.p4}} products and {{.p5}} boards; clean them up first",
		},
		{
			ID:    "请先配置执行动作",
			Other: "Configure actions first",
		},
		{
			ID:    "请先配置触发条件",
			Other: "Configure trigger conditions first",
		},
		{
			ID:    "请求体格式有误：{{.p1}}",
			Other: "Invalid request body: {{.p1}}",
		},
		{
			ID:    "请选择数据资源",
			Other: "Select a data resource",
		},
		{
			ID:    "请配置定时表达式",
			Other: "Configure a schedule expression",
		},
		{
			ID:    "请配置执行动作",
			Other: "Configure an action",
		},
		{
			ID:    "请配置触发条件",
			Other: "Configure a trigger condition",
		},
		{
			ID:    "读取网关列表失败: {{.p1}}",
			Other: "Failed to read the gateway list: {{.p1}}",
		},
		{
			ID:    "连接参数 {{.p1}} 过长（最多 {{.p2}} 个字符）",
			Other: "Connection parameter {{.p1}} is too long (at most {{.p2}} characters)",
		},
		{
			ID:    "连接参数项过多（最多 {{.p1}} 项）",
			Other: "Too many connection parameters (at most {{.p1}})",
		},
		{
			ID:    "配置必须是合法 JSON",
			Other: "Configuration must be valid JSON",
		},
		{
			ID:    "配置无法序列化：{{.p1}}",
			Other: "Configuration cannot be serialized: {{.p1}}",
		},
		{
			ID:    "配置过大（上限 {{.p1}} 字节）",
			Other: "Configuration is too large (limit {{.p1}} bytes)",
		},
		{
			ID:    "非法的参考系统主题: {{.p1}}",
			Other: "Invalid reference-system topic: {{.p1}}",
		},
		{
			ID:    "非法的网关主题: {{.p1}}",
			Other: "Invalid gateway topic: {{.p1}}",
		},

		// 后端接口返回的细节文案（errort.NewMessageErr，ID 就是中文原文）
		{
			ID:    "TDengine 返回错误: {{.p1}}",
			Other: "TDengine returned an error: {{.p1}}",
		},
		{
			ID:    "URL 必须以 http:// 或 https:// 开头",
			Other: "URL must start with http:// or https://",
		},
		{
			ID:    "{{.p1}} 不可达: {{.p2}}",
			Other: "{{.p1}} is unreachable: {{.p2}}",
		},
		{
			ID:    "响应解析失败: {{.p1}}",
			Other: "Failed to parse the response: {{.p1}}",
		},
		{
			ID:    "地址为空",
			Other: "Address is empty",
		},
		{
			ID:    "地址缺少端口: {{.p1}}",
			Other: "Address is missing a port: {{.p1}}",
		},
		{
			ID:    "服务返回状态码 {{.p1}}",
			Other: "The service returned status code {{.p1}}",
		},
		{
			ID:    "目标地址 {{.p1}} 属于云元数据/链路本地网段，禁止访问",
			Other: "Target address {{.p1}} is in a cloud metadata/link-local range and access is denied",
		},
		{
			ID:    "目标地址为空",
			Other: "Target address is empty",
		},
		{
			ID:    "缺少 URL",
			Other: "Missing URL",
		},
		{
			ID:    "缺少 brokers",
			Other: "Missing brokers",
		},
		{
			ID:    "缺少主机地址",
			Other: "Missing host address",
		},
		{
			ID:    "缺少服务地址",
			Other: "Missing service address",
		},
		{
			ID:    "该产品已绑定子设备，请优先删除子设备",
			Other: "This product has bound sub-devices; delete the sub-devices first",
		},
		{
			ID:    "请先删除绑定此配置的驱动",
			Other: "Delete the drivers bound to this configuration first",
		},
		{
			ID:    "请求失败: {{.p1}}",
			Other: "Request failed: {{.p1}}",
		},
		{
			ID:    "请求构造失败: {{.p1}}",
			Other: "Failed to build the request: {{.p1}}",
		},
		{
			ID:    "连接失败: {{.p1}}",
			Other: "Connection failed: {{.p1}}",
		},
		{
			ID:    "连接超时",
			Other: "Connection timed out",
		},
		{
			ID:    "验证任务排队超时，请稍后重试",
			Other: "Timed out waiting for the verification task, please try again later",
		},
		{
			ID:    "验证成功",
			Other: "Verification succeeded",
		},
		{
			ID:    "验证超时",
			Other: "Verification timed out",
		},

		// 告警通知正文（钉钉 / 企业微信）
		{
			ID:    "设备数据触发",
			Other: "Device data trigger",
		},
		{
			ID:    "设备事件触发",
			Other: "Device event trigger",
		},
		{
			ID:    "设备状态触发",
			Other: "Device status trigger",
		},
		{
			ID:    "在线",
			Other: "Online",
		},
		{
			ID:    "离线",
			Other: "Offline",
		},

		// 告警通知正文（钉钉 / 企业微信）
		{
			ID:    "设备已触发数据告警\n         >设备名称:<font color=\\\"comment\\\">{{.p1}}</font>\n         >告警规则:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >属性名称:<font color=\\\"comment\\\">{{.p3}}</font>\n \t\t >触发时间:<font color=\\\"comment\\\">{{.p4}}</font>\n      ",
			Other: "Device data alarm triggered\n         >Device name:<font color=\\\"comment\\\">{{.p1}}</font>\n         >Alarm rule:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >Property:<font color=\\\"comment\\\">{{.p3}}</font>\n \t\t >Triggered at:<font color=\\\"comment\\\">{{.p4}}</font>\n      ",
		},
		{
			ID:    "设备已触发数据告警\n         >设备名称:<font color=\\\"comment\\\">{{.p1}}</font>\n         >告警规则:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >属性名称:<font color=\\\"comment\\\">{{.p3}}</font>\n         >当前属性值:<font color=\\\"comment\\\">{{.p4}}</font>\n \t\t >触发时间:<font color=\\\"comment\\\">{{.p5}}</font>\n      ",
			Other: "Device data alarm triggered\n         >Device name:<font color=\\\"comment\\\">{{.p1}}</font>\n         >Alarm rule:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >Property:<font color=\\\"comment\\\">{{.p3}}</font>\n         >Current value:<font color=\\\"comment\\\">{{.p4}}</font>\n \t\t >Triggered at:<font color=\\\"comment\\\">{{.p5}}</font>\n      ",
		},
		{
			ID:    "设备已触发事件告警\n         >设备名称:<font color=\\\"comment\\\">{{.p1}}</font>\n         >告警规则:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >事件名称:<font color=\\\"comment\\\">{{.p3}}</font>\n \t     >触发时间:<font color=\\\"comment\\\">{{.p4}}</font>\n         ",
			Other: "Device event alarm triggered\n         >Device name:<font color=\\\"comment\\\">{{.p1}}</font>\n         >Alarm rule:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >Event:<font color=\\\"comment\\\">{{.p3}}</font>\n \t     >Triggered at:<font color=\\\"comment\\\">{{.p4}}</font>\n         ",
		},
		{
			ID:    "设备已触发事件告警\n         >设备名称:<font color=\\\"comment\\\">{{.p1}}</font>\n         >告警规则:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >事件名称:<font color=\\\"comment\\\">{{.p3}}</font>\n         >事件内容:<font color=\\\"comment\\\">{{.p4}}</font>\n \t     >触发时间:<font color=\\\"comment\\\">{{.p5}}</font>\n         ",
			Other: "Device event alarm triggered\n         >Device name:<font color=\\\"comment\\\">{{.p1}}</font>\n         >Alarm rule:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >Event:<font color=\\\"comment\\\">{{.p3}}</font>\n         >Event content:<font color=\\\"comment\\\">{{.p4}}</font>\n \t     >Triggered at:<font color=\\\"comment\\\">{{.p5}}</font>\n         ",
		},
		{
			ID:    "设备已触发状态告警\n         >设备名称:<font color=\\\"comment\\\">{{.p1}}</font>\n         >告警规则:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >设备状态:<font color=\\\"comment\\\">{{.p3}}</font>",
			Other: "Device status alarm triggered\n         >Device name:<font color=\\\"comment\\\">{{.p1}}</font>\n         >Alarm rule:<font color=\\\"comment\\\">{{.p2}}</font>\n\t\t >Device status:<font color=\\\"comment\\\">{{.p3}}</font>",
		},
		{
			ID:    "# 设备触发告警 \n\n设备名称：{{.p1}} \n\n告警规则名称：{{.p2}} \n\n触发方式：{{.p3}} \n\n",
			Other: "# Device alarm triggered \n\nDevice name: {{.p1}} \n\nAlarm rule: {{.p2}} \n\nTrigger: {{.p3}} \n\n",
		},

		// 后端接口返回的细节文案（errort.NewMessageErr，ID 就是中文原文）
		{
			ID:    "不支持的语言: {{.p1}}",
			Other: "Unsupported language: {{.p1}}",
		},
	}
}
