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
 * @author Andy Chow <diudiu8848@163.com>
 * websocket router
 * - one ws uri, one router
 * - one router, one manager
 */

type (
	//router inter config
	RouterCfg struct {
		Name             string
		Uri              string
		MsgType          int
		BufferSize       int
		Buckets          int
		HeartByte        []byte
		HeartRate        int //heart beat check rate, 0:no check
		CheckActiveRate  int //if 0 means not need check
		MaxActiveSeconds int
	}
)

//router info
type Router struct {
	rc          *RouterCfg //reference cfg
	connManager IConnManager
	cd          ICoder
	upGrader    websocket.Upgrader //ws up grader

	//cb func
	cbForGenConnId func() int64
	cbForConnected func(routerName string, connId int64, ctx *gin.Context) error
	cbForClosed    func(routerName string, connId int64, ctx *gin.Context) error
	cbForRead      func(routerName string, connId int64, messageType int, message []byte, ctx *gin.Context) error
}

//construct
func NewRouter(rc *RouterCfg) *Router {
	//default value setup
	if rc.MsgType < define.MessageTypeOfJson {
		rc.MsgType = define.MessageTypeOfOctet
	}
	if rc.BufferSize <= 0 {
		rc.BufferSize = define.DefaultBuffSize
	}
	if rc.Buckets <= 0 {
		rc.Buckets = define.DefaultBuckets
	}

	//setup upgrade
	//up grader for http -> websocket
	upGrader := websocket.Upgrader{
		ReadBufferSize:    define.DefaultBuffSize,
		WriteBufferSize:   define.DefaultBuffSize,
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	//setup read and write buffer size
	if rc.BufferSize > 0 {
		upGrader.ReadBufferSize = rc.BufferSize
		upGrader.WriteBufferSize = rc.BufferSize
	}

	//self init
	this := &Router{
		rc: rc,
		upGrader: upGrader,
		connManager: NewManager(rc.Buckets),
		cd:          NewCoder(),
	}

	//setup manager
	this.connManager.SetMessageType(rc.MsgType)
	if rc.HeartRate > 0 {
		this.connManager.SetHeartRate(rc.HeartRate)
	}
	return this
}

//close
func (f *Router) Close() {
	f.connManager.Close()
}

//set heart beat data
func (f *Router) SetHeartByte(data []byte) error {
	if data == nil {
		return errors.New("invalid parameter")
	}
	f.rc.HeartByte = data
	return nil
}

//set heart beat rate
func (f *Router) SetHeartRate(rate int) error {
	if rate < 0 {
		return errors.New("invalid parameter")
	}
	f.rc.HeartRate = rate
	return nil
}

//set message type
func (f *Router) SetMessageType(iType int) error {
	if iType < define.MessageTypeOfJson ||
		iType > define.MessageTypeOfOctet {
		return errors.New("invalid type")
	}
	f.rc.MsgType = iType
	f.connManager.SetMessageType(iType)
	return nil
}

//set cb for gen connect id
func (f *Router) SetCBForGenConnId(cb func() int64) {
	if cb == nil {
		return
	}
	f.cbForGenConnId = cb
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
func (f *Router) GetUriPara(name string, ctx *gin.Context) string {
	return ctx.Param(name)
}

//get heart beat
func (f *Router) GetHeartByte() []byte {
	return f.rc.HeartByte
}

//new connect entry
func (f *Router) Entry(ctx *gin.Context) {
	var (
		newConnId int64
		m any = nil
	)
	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("Router:Entry, panic err:%v, stack:%v",
				err, string(debug.Stack()))
		}
	}()

	//get key data
	req := ctx.Request
	writer := ctx.Writer

	//upgrade http connect to ws connect
	conn, err := f.upGrader.Upgrade(writer, req, nil)
	if err != nil {
		//500 error
		log.Printf("Router:Entry, upgrade http to websocket failed, err:%v\n", err.Error())
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//gen new connect id
	if f.cbForGenConnId != nil {
		newConnId = f.cbForGenConnId()
	}else{
		newConnId = f.connManager.GenConnId()
	}

	//accept new connect
	wsConn, subErr := f.connManager.Accept(newConnId, conn)
	if subErr != nil {
		//accept failed
		log.Printf("Router:Entry, accept connect failed, err:%v\n", subErr.Error())
		err = f.connManager.CloseWithMessage(conn, define.MessageForNormalClosed)
		if err != nil {
			log.Printf("Router:Entry, manager closed failed, err:%v\n", err.Error())
		}
		if f.cbForClosed != nil {
			f.cbForClosed(f.rc.Name, newConnId, ctx)
		}
		return
	}

	//cb connect
	if f.cbForConnected != nil {
		err = f.cbForConnected(f.rc.Name, newConnId, ctx)
		if err != nil {
			log.Printf("Router:Entry, cbForConnected err:%v\n", err.Error())
			//call cb connected failed, force close connect
			f.connManager.CloseWithMessage(conn, err.Error())
			if f.cbForClosed != nil {
				f.cbForClosed(f.rc.Name, newConnId, ctx)
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

//get name
func (f *Router) GetName() string {
	return f.rc.Name
}

//////////////
//private func
//////////////

//process one connect request, include read, write, etc.
//run as son process, one conn one process
func (f *Router) processRequest(
			connId int64,
			wsConn IWSConn,
			ctx *gin.Context,
		) error {
	var (
		messageType int
		message []byte
		err error
		m any = nil
	)

	//check
	if connId <= 0 || wsConn == nil || ctx == nil {
		return errors.New("invalid parameter")
	}

	//defer
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("Router:processRequest panic, err:%v, stack:%v",
				subErr, string(debug.Stack()))
		}
		f.connManager.CloseConn(connId)
	}()

	//loop select
	for {
		//check
		if &wsConn == nil {
			log.Printf("Router:processRequest, ws connect is nil\n")
			return errors.New("ws connect is nil")
		}

		//read original websocket data from client side
		messageType, message, err = wsConn.Read()
		if err != nil {
			if err == io.EOF {
				log.Printf("Router:processRequest, read EOF need close.")
			}else{
				log.Printf("Router:processRequest, read err:%v", err.Error())
			}
			//connect closed
			if f.cbForClosed != nil {
				f.cbForClosed(f.rc.Name, connId, ctx)
			}
			return err
		}

		//heart beat data check
		if f.rc.HeartRate > 0 && f.rc.HeartByte != nil && message != nil {
			if bytes.Compare(f.rc.HeartByte, message) == 0 {
				//it's heart beat data
				f.connManager.HeartBeat(connId)
				continue
			}
		}

		//check and run cb for read message
		if f.cbForRead != nil {
			f.cbForRead(f.rc.Name, connId, messageType, message, ctx)
		}
	}
	return err
}