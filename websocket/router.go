package websocket

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"tubing/base"
	"tubing/define"
)

/*
 * websocket router
 * - one ws uri, one router
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
	c *gin.Context
	connManager *Manager
	sessionName string
	cbForClosed func(session string) error
	cbForRead func(session string, messageType int, message []byte) error
}

//construct
func NewRouter() *Router {
	this := &Router{
		connManager: NewManager(),
	}
	return this
}

//set cb func
func (f *Router) SetCBForClosed(cb func(session string) error) {
	if cb == nil {
		return
	}
	f.cbForClosed = cb
}

func (f *Router) SetCBForRead(cb func(session string, messageType int, message []byte) error) {
	if cb == nil {
		return
	}
	f.cbForRead = cb
}

//set session name
func (f *Router) SetSessionName(name string) error {
	if name == "" {
		return errors.New("invalid parameter")
	}
	f.sessionName = name
	return nil
}

//entry
func (f *Router) Entry(c *gin.Context) {
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Router:Entry, panic err:%v, stack:%v",
				err, string(debug.Stack()))
		}
	}()

	//set running context
	f.c = c

	//get key data
	req := c.Request
	writer := c.Writer

	//get key param
	session := c.Query(f.sessionName)
	//contentType := c.Query(define.QueryParaOfContentType)

	//setup net base data
	netBase := &base.NetBase{
		//ContentType: contentType,
		ClientIP: c.ClientIP(), //get client id
	}

	//upgrade http connect to ws connect
	conn, err := upGrader.Upgrade(writer, req, nil)
	if err != nil {
		//500 error
		log.Printf("Router:Entry, upgrade http to websocket failed, err:%v\n", err.Error())
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//accept new connect
	wsConn, err := f.connManager.Accept(session, conn)
	if err != nil {
		//accept failed
		log.Printf("Router:Entry, accept connect failed, err:%v\n", err.Error())
		err = f.connManager.CloseWithMessage(conn, define.MessageForNormalClosed)
		if err != nil {
			log.Printf("Router:Entry, err:%v\n", err.Error())
		}
		if f.cbForClosed != nil {
			f.cbForClosed(session)
		}
		return
	}

	//spawn son process for request
	go f.processRequest(session, wsConn, netBase)
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
			session string,
			wsConn IWSConn,
			nb *base.NetBase,
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
		f.connManager.CloseConn(session)
	}()

	//loop select
	for {
		//read original websocket data
		messageType, message, err = wsConn.Read()
		if err != nil {
			if err == io.EOF {
				log.Printf("Router:processRequest, read EOF need close.")
				return
			}
			log.Printf("Router:processRequest, read err:%v", err.Error())
			return
		}
		//check and run cb for read message
		if f.cbForRead != nil {
			f.cbForRead(session, messageType, message)
		}
	}
}