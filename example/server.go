package main

import (
	"errors"
	"github.com/andyzhou/tubing"
	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

//cb for ws first connect
func cbForConnected(session string, ctx *gin.Context) error {
	log.Printf("cbForConnected, session:%v\n", session)

	//get para
	paras := ctx.Params
	log.Printf("cbForConnected, paras:%v\n", paras)

	//cast history to new conn, todo..
	conn, err := tb.GetConn(session)
	if err != nil || conn == nil {
		log.Printf("cbForConnected, session:%v, get conn failed, err:%v\n", session, err)
	}
	messageType := 1
	message := []byte("welcome")
	err = conn.Write(messageType, message)
	log.Printf("cbForConnected, session:%v, send result:%v\n", session, err)
	return nil
}

//cb for ws close connect
func cbForClosed(session string) error {
	log.Printf("cbForClosed, session:%v\n", session)
	return nil
}

//cb for ws read message
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

	////set pattern names
	//patternNames := []string{
	//	"module",
	//}

	//init service
	tb = tubing.GetServer()
	tb.SetGin(gin)
	tb.SetRootUriPattern("/ws")
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
	//init app
	app := &cli.App{
		Name: define.AppName,
		Action: func(c *cli.Context) error {
			return startApp(c)
		},
	}

	//start app
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("%v run failed, err:%v\n", define.AppName, err.Error())
		return
	}
}
