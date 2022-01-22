package cmd

/*
 * command flag fields
 */

//flag field name
const (
	NameOfHttpPort 	= "http_port"
	NameOfTcpPort 	= "tcp_port"
	NameOfConfPath 	= "config"
	NameOfEnv 		= "env"
)

//command config define
type (
	AppEnvConfig struct {
		HttpPort int
		TcpPort int
		ConfigPath string
		Env string
	}
)

func (c *AppEnvConfig) GetHttpPort() int {
	return c.HttpPort
}

func (c *AppEnvConfig) GetTcpPort() int {
	return c.TcpPort
}

func (c *AppEnvConfig) GetConfigPath() string {
	return c.ConfigPath
}

func (c *AppEnvConfig) GetEnv() string {
	return c.Env
}

