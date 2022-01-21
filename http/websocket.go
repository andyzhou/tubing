package http

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"tubing/base"
	"tubing/define"
	ws "tubing/websocket"
)

/*
 * websocket server
 * - http base on github.com/gin-gonic/gin
 * - ws base one github.com/gorilla/websocket
 */

//server info
type WebSocket struct {
	wsRootUri string
	gin *gin.Engine //parent reference
	upgrade websocket.Upgrader
	connManager ws.IConnManager
	coder ws.ICoder
}

//construct
func NewWebSocket() *WebSocket {
	this := &WebSocket{
		connManager: ws.NewManager(),
		coder: ws.NewCoder(),
	}
	return this
}

//init
func (f *WebSocket) Init(gin *gin.Engine, rootUri ...string) {
	//set gin and websocket root uri
	wsRootUri := define.WebSocketRoot
	if rootUri != nil && len(rootUri) > 0 {
		wsRootUri = rootUri[0]
	}
	f.wsRootUri = wsRootUri
	f.gin = gin
	//inter init
	f.interInit()
}

//get root uri
func (f *WebSocket) GetRootUri() string {
	return f.wsRootUri
}

//process connect
func (f *WebSocket) ProcessConn(c *gin.Context) {
	f.processConn(c)
}

////////////////
//private func
////////////////

//process web socket connect
func (f *WebSocket) processConn(c *gin.Context) {
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketServer:processRequest panic, err:%v, stack:%v",
						err, string(debug.Stack()))
		}
	}()

	//get key data
	request := c.Request
	writer := c.Writer

	//get key param
	session := c.Query(define.QueryParaOfSession)
	contentType := c.Query(define.QueryParaOfContentType)

	//setup net base data
	netBase := &base.NetBase{
		ContentType: contentType,
		ClientIP: c.ClientIP(), //get client id
	}

	//upgrade http connect to ws connect
	conn, err := f.upgrade.Upgrade(writer, request, nil)
	if err != nil {
		//500 error
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//accept new connect
	wsConn, err := f.connManager.Accept(session, conn)
	if err != nil {
		err = f.connManager.CloseWithMessage(conn, define.MessageForNormalClosed)
		if err != nil {
			log.Printf("WebSocketServer:processRequest, err:%v", err.Error())
		}
		return
	}

	//spawn son process for request
	go f.processRequest(session, wsConn, netBase)
}

//process request, include read, write, etc.
//run as son process
func (f *WebSocket) processRequest(session string, wsConn ws.IWSConn, nb *base.NetBase) {
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketServer:processRequest panic, err:%v, stack:%v",
						err, string(debug.Stack()))
		}
		f.connManager.CloseConn(session)
	}()

	//loop select
	for {
		//read original websocket data
		_, _, err := wsConn.Read()
		if err != nil {
			if err == io.EOF {
				log.Printf("WebSocketServer:processRequest, read EOF need close.")
				return
			}
			log.Printf("WebSocketServer:processRequest, read err:%v", err.Error())
			return
		}
		//try unmarshal message data
	}
}

//inter init
func (f *WebSocket) interInit() {
	//init websocket upgrade
	f.upgrade = websocket.Upgrader{
		ReadBufferSize: define.WebSocketBufferSize,
		WriteBufferSize: define.WebSocketBufferSize,
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	//setup websocket
	if f.gin == nil {
		panic("WebSocket:interInit, gin instance is nil")
		return
	}
}

