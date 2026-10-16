package constants

const (
	DefaultDriverBaseAddress = "localhost:-1"
	HostAddress              = "127.0.0.1"

	// HostDockerInternal 宿主机在容器内的别名。
	// 平台不在容器里（本机进程模式）时，驱动容器要靠它连回平台：
	// macOS/Windows 的 Docker Desktop 原生支持；
	// Linux 由 ContainerStart 注入 host-gateway 别名（见 dmi/docker/manager.go 的 ExtraHosts）。
	HostDockerInternal = "host.docker.internal"

	ConfigSuffix = ".toml"

	ConfigKeyDriver = "driver"

	DriverBaseDir            = "driver-data" // 驱动相关目录dir
	DriverLibraryDir         = DriverBaseDir + "/driver-library"
	DriverBinDir             = DriverBaseDir + "/bin"
	DriverRunConfigDir       = DriverBaseDir + "/run-config"
	DriverMntDir             = DriverBaseDir + "/mnt"
	DriverDefaultLogPath     = "logs/driver.log"
	DockerIotServerRootDir = "/var/bin/iotserver"
)

const (
	DriverLibTypeDefault = iota + 1
	DriverLibTypeAppService
)

const (
	DeviceLibraryUploadTypeConfig = 2

	// 驱动配置定义，key=>value 的type
	DriverConfigTypeInt    = "int"
	DriverConfigTypeFloat  = "float"
	DriverConfigTypeString = "string"
	DriverConfigTypeBool   = "bool"
	DriverConfigTypeSelect = "select"
	DriverConfigTypeObject = "object"
	DriverConfigTypeArray  = "array"
)
