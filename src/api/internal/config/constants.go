package config

// ConfigFlagName 是启动参数和命令说明共用的配置选项名。
const ConfigFlagName = "config"

const (
	defaultConfigFilename = "config.yaml"
	defaultTimeZone       = "Asia/Shanghai"
	minimumTCPPort        = 1
	maximumTCPPort        = 65535
)
