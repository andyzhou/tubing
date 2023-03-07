package cmd

import (
	"github.com/urfave/cli"
	"sync"
	"github.com/andyzhou/tubing/define"
)

/*
 * command flags
 */

var (
	appEnvConfig *AppEnvConfig
	appConfigOnce sync.Once
)

//gather relate cmd flags
func Flags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{Name: NameOfHttpPort, Value: define.AppHttpPort, Usage: "http port"},
		&cli.IntFlag{Name: NameOfTcpPort, Value: define.AppTcpPort, Usage: "tcp port"},
		&cli.StringFlag{Name: NameOfConfPath, Value: define.DefaultIniPath, Usage: "config path"},
		&cli.StringFlag{Name: NameOfEnv, Value: define.EnvOfDev, Usage: "running env"},
	}
}

//get app env config single instance
func GetEnvConfig() *AppEnvConfig {
	return appEnvConfig
}
func GetEnvConfigOnce(c *cli.Context) *AppEnvConfig {
	appConfigOnce.Do(func() {
		appEnvConfig = &AppEnvConfig{
			HttpPort: c.Int(NameOfHttpPort),
			TcpPort: c.Int(NameOfTcpPort),
			ConfigPath: c.String(NameOfConfPath),
			Env: c.String(NameOfEnv),
		}
	})
	return appEnvConfig
}