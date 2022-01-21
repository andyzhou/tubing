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
	AppConfig struct {
		HttpPort int
		TcpPort int
		ConfigPath string
		Env string
	}
)

func (c *AppConfig) GetHttpPort() int {
	return c.HttpPort
}

func (c *AppConfig) GetTcpPort() int {
	return c.TcpPort
}

func (c *AppConfig) GetConfigPath() string {
	return c.ConfigPath
}

func (c *AppConfig) GetEnv() string {
	return c.Env
}

