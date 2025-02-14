package main

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing"
	"github.com/andyzhou/tubing/define"
	eDefine "github.com/andyzhou/tubing/example/define"
	"github.com/andyzhou/tubing/example/json"
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
	RouterUri  = "/ws"
	Buckets = 1
	ServerPort = 8090
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
func cbForConnected(
	routerName string,
	connId int64,
	conn face.IWSConn,
	ctx *gin.Context) error {
	//log.Printf("cbForConnected, connId:%v\n", connId)

	//get para
	//namePara := tubing.GetServer().GetPara("name", ctx)
	//log.Printf("cbForConnected, namePara:%v\n", namePara)

	//get router
	router, err := getRouterByName(routerName)
	if err != nil {
		return err
	}
	if router == nil {
		return errors.New("invalid router name")
	}

	//reply welcome message
	messageType := define.MessageTypeOfJson
	message := []byte("welcome you!")
	err = conn.Write(messageType, message)
	if err != nil {
		log.Printf("cbForConnected, connId:%v, send result:%v\n", connId, err)
	}
	return nil
}

//cb for ws close connect
func cbForClosed(
	routerName string,
	connId int64,
	conn face.IWSConn,
	ctx *gin.Context) error {
	log.Printf("cbForClosed, connId:%v\n", connId)
	return nil
}

//cb for ws read message
func cbForRead(
	routerName string,
	connId int64,
	conn face.IWSConn,
	messageType int,
	message []byte,
	ctx *gin.Context) error {
	if tb == nil {
		return errors.New("tb not init yet")
	}
	log.Printf("cbForRead, connId:%v, messageType:%v, message:%v\n",
				connId, messageType, string(message))

	//decode message
	messageObj := json.NewMessageJson()
	err := messageObj.Decode(message, messageObj)
	if err != nil {
		return err
	}

	//get router
	router, subErr := getRouterByName(routerName)
	if subErr != nil {
		return subErr
	}
	if router == nil {
		return errors.New("invalid router name")
	}

	//do opt by message kind
	switch messageObj.Kind {
	case eDefine.MsgKindOfLogin:
		{
			//user login
			//decode login obj
			genObjMap, _ := messageObj.JsonObj.(map[string]interface{})
			jsonObjByte, _ := messageObj.EncodeSimple(genObjMap)
			loginObj := json.NewLoginJson()
			loginObj.Decode(jsonObjByte, loginObj)

			log.Printf("user login, conn id:%v, userId:%v, userNick:%v\n",
				connId, loginObj.Id, loginObj.Nick)

			//set conn property
			if conn != nil && loginObj != nil {
				conn.SetProp(eDefine.PropNameOfUserId, loginObj.Id)
				conn.SetProp(eDefine.PropNameOfUserNick, loginObj.Nick)
			}
			break
		}
	case eDefine.MsgKindOfChat:
		{
			//chat message
			//decode message obj
			genObjMap, _ := messageObj.JsonObj.(map[string]interface{})
			jsonObjByte, _ := messageObj.EncodeSimple(genObjMap)
			chatObj := json.NewChatJson()
			chatObj.Decode(jsonObjByte, chatObj)

			//get conn property
			//conn, _ := router.GetManager().GetConn(connId)
			if conn != nil && chatObj != nil {
				userNick, _ := conn.GetProp(eDefine.PropNameOfUserNick)
				if userNick != nil {
					userNickStr, _ := userNick.(string)
					chatObj.Sender = userNickStr
				}
				newChatBytes, _ := chatObj.Encode(chatObj)

				log.Printf("user chat, conn id:%v, sender:%v, msg:%v\n",
					connId, chatObj.Sender, chatObj.Message)

				//setup send msg para
				sendMsgPara := &define.SendMsgPara{
					Msg: newChatBytes,
				}

				//cast to all
				err = router.GetManager().SendMessage(sendMsgPara)
				if err != nil {
					log.Println("cast chat message failed, err:", err.Error())
					return err
				}
			}
			break
		}
	default:
		{
			return fmt.Errorf("invalid message kind %v", messageObj.Kind)
		}
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
func createGin(isReleases ...bool) *gin.Engine {
	var (
		isRelease bool
	)
	//release mode check
	if isReleases != nil && len(isReleases) > 0 {
		isRelease = isReleases[0]
	}
	if isRelease {
		gin.SetMode(gin.ReleaseMode)
	}

	//init default gin router and page
	ginRouter := gin.New()
	if isRelease {
		ginRouter.Use(gin.Recovery())
	}

	//init templates
	ginRouter.LoadHTMLGlob("./tpl/*.tpl")

	//init static path
	ginRouter.Static("/html", "./html")

	//register home request url
	ginRouter.Any("/", showHomePage)
	return ginRouter
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
	gin := createGin(true)

	//set router
	ur := &tubing.UriRouter{
		RouterName: RouterName,
		RouterUri: RouterUri,
		Buckets: Buckets,
		MsgType: define.MessageTypeOfJson,
		ReadByteRate: 0.01, //xx seconds
		//CheckActiveRate: 300, //xx seconds
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
	err = tb.StartGin(ServerPort, true)
	if err != nil {
		return err
	}
	log.Printf("start %v on port %v done..\n", c.App.Name, ServerPort)
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
