package main

import (
	"errors"
	"github.com/andyzhou/tubing"
	"github.com/andyzhou/tubing/define"
	"github.com/andyzhou/tubing/face"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	RouterName = "test"
	RouterUri = "/ws"
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
func cbForConnected(routerName string, connId int64, ctx *gin.Context) error {
	log.Printf("cbForConnected, connId:%v\n", connId)

	//get para
	paras := ctx.Params
	log.Printf("cbForConnected, paras:%v\n", paras)

	//get router
	router, err := getRouterByName(routerName)
	if err != nil {
		return err
	}
	if router == nil {
		return errors.New("invalid router name")
	}

	//cast test welcome message
	conn, err := router.GetManager().GetConn(connId)
	if err != nil || conn == nil {
		log.Printf("cbForConnected, connId:%v, get conn failed, err:%v\n", connId, err)
	}
	messageType := 1
	message := []byte("welcome you!")
	err = conn.Write(messageType, message)
	log.Printf("cbForConnected, connId:%v, send result:%v\n", connId, err)
	return nil
}

//cb for ws close connect
func cbForClosed(routerName string, connId int64, ctx *gin.Context) error {
	log.Printf("cbForClosed, connId:%v\n", connId)
	return nil
}

//cb for ws read message
func cbForRead(routerName string, connId int64, messageType int, message []byte, ctx *gin.Context) error {
	log.Printf("cbForRead, connId:%v, messageType:%v, message:%v\n",
		connId, messageType, string(message))
	if tb == nil {
		return errors.New("tb not init yet")
	}

	//get router
	router, err := getRouterByName(routerName)
	if err != nil {
		return err
	}
	if router == nil {
		return errors.New("invalid router name")
	}

	//cast to all
	err = router.GetManager().CastMessage(message)
	if err != nil {
		log.Println("cast message failed, err:", err.Error())
		return err
	}
	return nil
}

//get router by name
func getRouterByName(name string) (face.IRouter, error) {
	return tb.GetRouter(name)
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
		RouterName: RouterName,
		RouterUri: RouterUri,
		HeartByte: []byte(define.MessageBodyOfHeartBeat),
		CBForConnected: cbForConnected,
		CBForClosed: cbForClosed,
		CBForRead: cbForRead,
	}

	//init service
	tb = tubing.GetServer()
	tb.SetGin(gin)
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
