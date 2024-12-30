package face

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket router
 * - one ws uri, one router
 * - one manager, batch workers
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
		ReadByteRate	 float64 //read ws data rate
		SendByteRate	 float64 //write ws data rate
		CheckActiveRate  int //if 0 means not need check
		MaxActiveSeconds int
	}
)

//router info
type Router struct {
	rc          *RouterCfg //reference cfg
	connManager IManager
	cd          ICoder
	upGrader    websocket.Upgrader //ws up grader

	//cb func
	cbForGenConnId func() int64
	cbForConnected func(routerName string, connId int64, conn IWSConn, ctx *gin.Context) error
	cbForClosed    func(routerName string, connId int64, conn IWSConn, ctx *gin.Context) error
	cbForRead      func(routerName string, connId int64, conn IWSConn, messageType int, message []byte, ctx *gin.Context) error
}

//construct
func NewRouter(rc *RouterCfg) *Router {
	//default value setup
	if rc.MsgType < define.MessageTypeOfJson ||
		rc.MsgType > define.MessageTypeOfOctet {
		rc.MsgType = define.MessageTypeOfJson
	}
	if rc.BufferSize <= 0 {
		rc.BufferSize = define.DefaultBuffSize
	}
	if rc.Buckets <= 0 {
		rc.Buckets = define.DefaultBuckets
	}
	if rc.ReadByteRate <= 0 {
		rc.ReadByteRate = define.DefaultReadDataRate
	}

	//setup http upgrade
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
		rc:          rc,
		upGrader:    upGrader,
		cd:          NewCoder(),
	}

	//init manager
	this.connManager = NewManager(this)
	return this
}

//close
func (f *Router) Quit() {
	f.connManager.Quit()
}

//set heart beat data
func (f *Router) SetHeartByte(data []byte) error {
	if data == nil {
		return errors.New("invalid parameter")
	}
	f.rc.HeartByte = data
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
func (f *Router) SetCBForConnected(cb func(routerName string, connId int64, conn IWSConn, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForConnected = cb
}

//set cb func for conn closed
func (f *Router) SetCBForClosed(cb func(routerName string, connId int64, conn IWSConn, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForClosed = cb
	f.GetManager().SetCBForConnClosed(cb)
}

//set cb func for read data
func (f *Router) SetCBForRead(cb func(routerName string, connId int64, conn IWSConn, messageType int, message []byte, ctx *gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForRead = cb
	f.GetManager().SetCBForReadMessage(cb)
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

//get router config
func (f *Router) GetRouterCfg() *RouterCfg {
	return f.rc
}

//new connect entry
//need init gin context reference!!!
func (f *Router) Entry(ctx *gin.Context) {
	var (
		newConnId int64
		m any = nil
	)
	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("Router:Entry, panic err:%v, stack:%v", err, string(debug.Stack()))
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
	if newConnId <= 0 {
		err = errors.New("can't gen new connect id")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	//accept new connect
	wsConn, subErr := f.connManager.Accept(newConnId, conn, ctx)
	if subErr != nil {
		//accept failed
		log.Printf("Router:Entry, accept connect failed, err:%v\n", subErr.Error())
		err = f.connManager.CloseWithMessage(conn, define.MessageForNormalClosed)
		if err != nil {
			log.Printf("Router:Entry, manager closed failed, err:%v\n", err.Error())
		}
		if f.cbForClosed != nil {
			f.cbForClosed(f.rc.Name, newConnId, wsConn, ctx)
		}
		return
	}

	//cb connect, delay opt for spawn new connect init
	df := func() {
		if f.cbForConnected != nil {
			err = f.cbForConnected(f.rc.Name, newConnId, wsConn, ctx)
			if err != nil {
				log.Printf("Router:Entry, cbForConnected err:%v\n", err.Error())
				//call cb connected failed, force close connect
				f.connManager.CloseWithMessage(conn, err.Error())
				if f.cbForClosed != nil {
					f.cbForClosed(f.rc.Name, newConnId, wsConn, ctx)
				}
				return
			}
		}
	}
	time.AfterFunc(time.Second/10, df)
}

//get connect manager
func (f *Router) GetManager() IManager {
	return f.connManager
}

//get name
func (f *Router) GetName() string {
	return f.rc.Name
}

//get config
func (f *Router) GetConf() *RouterCfg {
	return f.rc
}