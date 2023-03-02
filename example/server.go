package main

import (
	"github.com/andyzhou/tubing/define"
	"github.com/urfave/cli"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"tubing"
	"tubing/lib/cmd"
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

//cb for ws
func cbForConn(session string) error {
	return nil
}
func cbForClosed(session string) error {
	return nil
}
func cbForRead(session string, messageType int, message []byte) error {
	return nil
}

//start app service
func startApp(c *cli.Context) error {
	var (
		wg sync.WaitGroup
	)
	//spawn signal process
	go signalProcess()
	log.Printf("start %v..\n", c.App.Name)

	//get app env config
	//appEnvConf := cmd.GetEnvConfigOnce(c)

	////init inter servers
	//server := server.NewServer(appEnvConf)
	//server.Start()

	//set router
	ur := &tubing.UriRouter{
		SessionName: define.QueryParaOfSession,
		CBForConn: cbForConn,
		CBForRead: cbForRead,
		CBForClosed: cbForClosed,
	}

	//init service
	tb := tubing.GetServer()
	tb.SetRootUri("/ws")
	err := tb.RegisterUri(ur)
	if err != nil {
		return err
	}

	//try start service
	wg.Add(1)
	err = tb.StartGin(8090)
	if err != nil {
		return err
	}
	log.Printf("start %v done..\n", c.App.Name)
	wg.Wait()
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
		log.Fatalf("%v run failed, err:%v\n", define.AppName, err.Error())
		return
	}
}
