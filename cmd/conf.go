package cmd

import (
	"github.com/urfave/cli"
	"sync"
	"tubing/define"
)

/*
 * command flags
 */

var (
	appConfig *AppConfig
	appConfigOnce sync.Once
)

//gather relate cmd flags
func Flags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{Name: NameOfHttpPort, Value: define.AppHttpPort, Usage: "http port"},
		&cli.IntFlag{Name: NameOfTcpPort, Value: define.AppTcpPort, Usage: "tcp port"},
		&cli.StringFlag{Name: NameOfConfPath, Value: "./etc", Usage: "config path"},
		&cli.StringFlag{Name: NameOfEnv, Value: define.EnvOfDev, Usage: "running env"},
	}
}

//get app config single instance
func GetAppConfig(c *cli.Context) *AppConfig {
	appConfigOnce.Do(func() {
		appConfig = &AppConfig{
			HttpPort: c.Int(NameOfHttpPort),
			TcpPort: c.Int(NameOfTcpPort),
			ConfigPath: c.String(NameOfConfPath),
			Env: c.String(NameOfEnv),
		}
	})
	return appConfig
}