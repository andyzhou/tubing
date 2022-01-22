package main

import (
	"github.com/urfave/cli"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tubing/define"
	"tubing/lib/cmd"
	"tubing/server"
)

//signal watch
func signalProcess() {
	c := make(chan os.Signal, 1)
	signal.Notify(
				c,
				syscall.SIGINT,
				syscall.SIGKILL,
				syscall.SIGTERM,
				syscall.SIGQUIT,
				syscall.SIGSTOP,
			)
	s := <- c
	log.Printf("get signal:%v", s.String())
	os.Exit(1)
}

//start app service
func startApp(c *cli.Context) error {
	//spawn signal process
	go signalProcess()
	log.Printf("start %v..\n", c.App.Name)

	//get app env config
	appEnvConf := cmd.GetEnvConfigOnce(c)

	//init inter servers
	server := server.NewServer(appEnvConf)
	server.Start()
	return nil
}

func main() {
	//get command flags
	flags := cmd.Flags()

	//init app
	app := &cli.App{
		Name: define.AppName,
		Action: func(c *cli.Context) error {
			return startApp(c)
		},
		Flags: flags,
	}

	//start app
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("%v run failed, err:%v", define.AppName, err.Error())
		return
	}
}
