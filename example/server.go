package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"tubing"
	"tubing/define"
	"tubing/lib/cmd"
)

var (
	tb *tubing.Server
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
func cbForConnected(session string, para map[string]interface{}) error {
	log.Printf("cbForConnected, session:%v, para:%v\n", session, para)
	return nil
}

func cbForClosed(session string) error {
	log.Printf("cbForClosed, session:%v\n", session)
	return nil
}

func cbForRead(session string, messageType int, message []byte) error {
	log.Printf("cbForRead, session:%v, messageType:%v, message:%v\n",
		session, messageType, string(message))
	if tb == nil {
		return errors.New("tb not init yet")
	}
	//cast to all
	err := tb.CastMessage(messageType, message)
	if err != nil {
		log.Println("cast message failed, err:", err.Error())
		return err
	}
	return nil
}

//show home page
func showHomePage(ctx *gin.Context) {
	//out put page
	tplFile := "chat.tpl"
	ctx.HTML(http.StatusOK, tplFile, nil)
}

//create default gin
func createGin() *gin.Engine {
	//init default gin and page
	gin := gin.Default()

	//init templates
	gin.LoadHTMLGlob("./tpl/*.tpl")

	//init static path
	gin.Static("/html", "./html")

	//register home request url
	gin.Any("/", showHomePage)
	return gin
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

	//init default gin
	gin := createGin()

	//set router
	ur := &tubing.UriRouter{
		SessionName: define.QueryParaOfSession,
		CBForConnected: cbForConnected,
		CBForClosed: cbForClosed,
		CBForRead: cbForRead,
	}

	//init service
	tb = tubing.GetServer()
	tb.SetGin(gin)
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
