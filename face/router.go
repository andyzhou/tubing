package face

import (
	"bytes"
	"errors"
	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"runtime/debug"
)

/*
 * websocket router
 * - one ws uri, one router
 * - one router, one manager
 */

var (
	//up grader for http -> websocket
	upGrader = websocket.Upgrader{
		ReadBufferSize:    define.DefaultBuffSize,
		WriteBufferSize:   define.DefaultBuffSize,
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

//router info
type Router struct {
	name string
	uri string
	msgType int
	heartByte []byte //heart beat data
	c *gin.Context
	connManager IConnManager
	cd ICoder
	cbForConnected func(routerName string, connId int64, ctx *gin.Context) error
	cbForClosed func(routerName string, connId int64, ctx *gin.Context) error
	cbForRead func(routerName string, connId int64, messageType int, message []byte, ctx *gin.Context) error
}

//construct
func NewRouter(name, uri string) *Router {
	defaultMsgType := define.MessageTypeOfOctet
	this := &Router{
		name: name,
		uri: uri,
		msgType: defaultMsgType,
		connManager: NewManager(),
		cd:          NewCoder(),
	}
	this.connManager.SetMessageType(defaultMsgType)
	return this
}

//set heart beat data
func (f *Router) SetHeartByte(data []byte) error {
	if data == nil {
		return errors.New("invalid parameter")
	}
	f.heartByte = data
	return nil
}

//set message type
func (f *Router) SetMessageType(iType int) error {
	if iType < define.MessageTypeOfJson ||
		iType > define.MessageTypeOfOctet {
		return errors.New("invalid type")
	}
	f.msgType = iType
	return nil
}

//set cb func for connected
func (f *Router) SetCBForConnected(cb func(routerName string, connId int64, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForConnected = cb
}

//set cb func for conn closed
func (f *Router) SetCBForClosed(cb func(routerName string, connId int64, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForClosed = cb
}

//set cb func for read data
func (f *Router) SetCBForRead(cb func(routerName string, connId int64, messageType int, message []byte, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForRead = cb
}

//get coder
func (f *Router) GetCoder() ICoder {
	return f.cd
}

//get uri pattern para
func (f *Router) GetUriPara(name string) string {
	return f.c.Param(name)
}

//entry
func (f *Router) Entry(ctx *gin.Context) {
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Router:Entry, panic err:%v, stack:%v",
				err, string(debug.Stack()))
		}
	}()

	//set running context
	f.c = ctx

	//get key data
	req := ctx.Request
	writer := ctx.Writer

	//gen new connect id
	newConnId := f.connManager.GenConnId()

	//get all para
	//paraValMap := c.Request.URL.Query()
	////setup net base data
	//netBase := &base.NetBase{
	//	//ContentType: contentType,
	//	ClientIP: c.ClientIP(), //get client id
	//}

	//upgrade http connect to ws connect
	conn, err := upGrader.Upgrade(writer, req, nil)
	if err != nil {
		//500 error
		log.Printf("Router:Entry, upgrade http to websocket failed, err:%v\n", err.Error())
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//accept new connect
	wsConn, err := f.connManager.Accept(newConnId, conn)
	if err != nil {
		//accept failed
		log.Printf("Router:Entry, accept connect failed, err:%v\n", err.Error())
		err = f.connManager.CloseWithMessage(conn, define.MessageForNormalClosed)
		if err != nil {
			log.Printf("Router:Entry, err:%v\n", err.Error())
		}
		if f.cbForClosed != nil {
			f.cbForClosed(f.name, newConnId, ctx)
		}
		return
	}

	//cb connect
	if f.cbForConnected != nil {
		//paraMap := map[string]interface{}{}
		//for k, v := range paraValMap {
		//	paraMap[k] = v
		//}
		err = f.cbForConnected(f.name, newConnId, ctx)
		if err != nil {
			log.Printf("Router:Entry, cbForConnected err:%v\n", err.Error())
			//call cb connected failed, force close connect
			f.connManager.CloseWithMessage(conn, err.Error())
			if f.cbForClosed != nil {
				f.cbForClosed(f.name, newConnId, ctx)
			}
			return
		}
	}

	//spawn son process for request
	go f.processRequest(newConnId, wsConn, ctx)
}

//get connect manager
func (f *Router) GetManager() IConnManager {
	return f.connManager
}

//////////////
//private func
//////////////

//process request, include read, write, etc.
//run as son process, one conn one process
func (f *Router) processRequest(
			connId int64,
			wsConn IWSConn,
			ctx *gin.Context,
		) {
	var (
		messageType int
		message []byte
		err error
	)

	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Router:processRequest panic, err:%v, stack:%v",
				err, string(debug.Stack()))
		}
		f.connManager.CloseConn(connId)
	}()

	//loop select
	for {
		//read original websocket data
		messageType, message, err = wsConn.Read()
		if err != nil {
			if err == io.EOF {
				log.Printf("Router:processRequest, read EOF need close.")
			}else{
				log.Printf("Router:processRequest, read err:%v", err.Error())
			}
			//connect closed
			if f.cbForClosed != nil {
				f.cbForClosed(f.name, connId, ctx)
			}
			return
		}
		//heart beat data check
		if f.heartByte != nil && message != nil {
			if bytes.Compare(f.heartByte, message) == 0 {
				//it's heart beat data
				f.connManager.HeartBeat(connId)
				continue
			}
		}

		//check and run cb for read message
		if f.cbForRead != nil {
			log.Printf("msgType:%v, msg:%v\n", messageType, string(message))
			f.cbForRead(f.name, connId, messageType, message, ctx)
		}
	}
}