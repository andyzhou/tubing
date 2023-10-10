package tubing

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing/define"
	"github.com/andyzhou/tubing/face"
	"github.com/gin-gonic/gin"
	"sync"
)

/*
 * service interface
 */

//global variable
var (
	_tb *Server
	_tbOnce sync.Once
)

//info for router cb
type UriRouter struct {
	//base
	RouterName string
	RouterUri string
	MsgType int
	HeartByte []byte
	BuffSize int //read and write buffer size
	//relate cb func
	CBForConnected func(routerName string, connId int64, ctx *gin.Context) error
	CBForClosed func(routerName string, connId int64, ctx *gin.Context) error
	CBForRead func(routerName string, connId int64, messageType int, message []byte, ctx *gin.Context) error
}

//face info
type Server struct {
	gin     *gin.Engine
	routerUris sync.Map //uri -> name
	routers sync.Map //name -> IRouter, router interface
	started bool
}

//get single instance
func GetServer() *Server {
	_tbOnce.Do(func() {
		_tb = NewServer()
	})
	return _tb
}

//construct
func NewServer(gs ... *gin.Engine) *Server {
	var (
		g *gin.Engine
	)
	//check and set default gin engine
	if gs != nil && len(gs) > 0 {
		g = gs[0]
	}
	if g == nil {
		g = gin.Default()
	}
	//self init
	this := &Server{
		gin: g,
		routerUris: sync.Map{},
		routers: sync.Map{},
	}
	return this
}

//get router uri by name
func (f *Server) GetUri(name string) (string, error) {
	var (
		uri string
	)
	if name == "" {
		return "", errors.New("invalid parameter")
	}
	sf := func(k, v interface{}) bool {
		tempName, ok := v.(string)
		if ok && tempName != "" && tempName == name {
			name, _ = k.(string)
			return false
		}
		return true
	}
	f.routerUris.Range(sf)
	return uri, nil
}

//get router uri name
func (f *Server) GetUriName(uri string) (string, error) {
	if uri == "" {
		return "", errors.New("invalid parameter")
	}
	v, ok := f.routerUris.Load(uri)
	if !ok || v == nil {
		return "", nil
	}
	name, ok := v.(string)
	if !ok || name == "" {
		return "", nil
	}
	return name, nil
}

//get router by name
func (f *Server) GetRouter(name string) (face.IRouter, error) {
	if name == "" {
		return nil, errors.New("invalid parameter")
	}
	v, ok := f.routers.Load(name)
	if !ok || v == nil {
		return nil, errors.New("no such router name")
	}
	router, _ := v.(face.IRouter)
	return router, nil
}

//get message type
func (f *Server) GetMsgTypeOfJson() int {
	return define.MessageTypeOfJson
}
func (f *Server) GetMsgTypeOfByte() int {
	return define.MessageTypeOfOctet
}

//set gin
func (f *Server) SetGin(g *gin.Engine) error {
	if g == nil {
		return errors.New("invalid gin engine object")
	}
	f.gin = g
	return nil
}

//register websocket uri
//methods include `GET` or `POST`
func (f *Server) RegisterUri(ur *UriRouter, methods ...string) error {
	//check
	if ur == nil || ur.RouterName == "" || ur.RouterUri == "" {
		return errors.New("invalid parameter")
	}
	if f.gin == nil {
		return errors.New("inter gin engine not init yet")
	}
	name, _ := f.GetUriName(ur.RouterUri)
	if name != "" {
		return errors.New("router uri had exists")
	}

	//setup method
	method := ""
	if methods != nil && len(methods) > 0 {
		method = methods[0]
	}

	//setup router inter cfg
	rc := &face.RouterCfg{
		Name: ur.RouterName,
		Uri: ur.RouterUri,
		MsgType: ur.MsgType,
		BufferSize: ur.BuffSize,
		HeartByte: ur.HeartByte,
	}

	//init new router
	router := face.NewRouter(rc)

	//setup relate key data and callbacks
	if ur.HeartByte != nil {
		router.SetHeartByte(ur.HeartByte)
	}
	if ur.MsgType > 0 {
		router.SetMessageType(ur.MsgType)
	}

	//setup callback
	router.SetCBForConnected(ur.CBForConnected)
	router.SetCBForClosed(ur.CBForClosed)
	router.SetCBForRead(ur.CBForRead)

	//begin register
	switch method {
	case define.ReqMethodOfPost:
		f.gin.POST(ur.RouterUri, router.Entry)
	case define.ReqMethodOfGet:
		f.gin.GET(ur.RouterUri, router.Entry)
	default:
		f.gin.Any(ur.RouterUri, router.Entry)
	}

	//sync into map
	f.routerUris.Store(ur.RouterUri, ur.RouterName)
	f.routers.Store(ur.RouterName, router)
	return nil
}

//close
func (f *Server) Close() error {
	//try close all
	sf := func(k, v interface{}) bool {
		router, ok := v.(face.IRouter)
		if ok && router != nil {
			router.GetManager().Close()
			f.routers.Delete(k)
		}
		return true
	}
	f.routers.Range(sf)
	f.routers = sync.Map{}
	f.routerUris = sync.Map{}
	return nil
}

//start gin (optional)
//used for service mode
func (f *Server) StartGin(port int) error {
	//check
	if port <= 0 {
		return errors.New("invalid parameter")
	}
	if f.gin == nil {
		return errors.New("gin hadn't init yet")
	}
	if f.started {
		return errors.New("server had started")
	}
	//start server
	serverAddr := fmt.Sprintf(":%v", port)
	go f.gin.Run(serverAddr)
	f.started = true
	return nil
}