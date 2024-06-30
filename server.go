package tubing

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing/define"
	"github.com/andyzhou/tubing/face"
	"github.com/gin-gonic/gin"
	"runtime"
	"sync"
	"sync/atomic"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
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
	CheckActiveRate int //if 0 means not need check
	MaxActiveSeconds int

	//relate cb func
	CBForGenConnId func() int64
	CBForConnected func(routerName string, connId int64, ctx *gin.Context) error
	CBForClosed func(routerName string, connId int64, ctx ...*gin.Context) error
	CBForRead func(routerName string, connId int64, messageType int, message []byte) error
}

//face info
type Server struct {
	gin         *gin.Engine
	routerUris  sync.Map //uri -> name
	routers     sync.Map //name -> IRouter, router interface
	routerCount int32
	started     bool
}

//get single instance
func GetServer() *Server {
	_tbOnce.Do(func() {
		_tb = NewServer()
	})
	return _tb
}

//construct
func NewServer() *Server {
	//self init
	this := &Server{
		routerUris: sync.Map{},
		routers: sync.Map{},
	}
	return this
}

//quit
func (f *Server) Quit() {
	if f.routerCount <= 0 {
		return
	}
	//stop all routers
	sf := func(k, v interface{}) bool {
		router, ok := v.(face.IRouter)
		if ok && router != nil {
			router.Close()
			f.routers.Delete(k)
			atomic.AddInt32(&f.routerCount, -1)
		}
		return true
	}
	f.routers.Range(sf)
	if f.routerCount <= 0 {
		atomic.StoreInt32(&f.routerCount, 0)
	}
	f.routers = sync.Map{}
	f.routerUris = sync.Map{}

	//memory gc
	runtime.GC()
}

//remove one url by name
func (f *Server) RemoveUriByName(name string) error {
	//check
	if name == "" {
		return errors.New("invalid parameter")
	}

	//get router by name
	router, err := f.GetRouter(name)
	if err != nil {
		return err
	}
	if router == nil {
		return errors.New("no such router by name")
	}

	//get uri by name
	uri, _ := f.GetUri(name)

	//close
	router.Close()
	f.routers.Delete(name)
	if uri != "" {
		f.routerUris.Delete(uri)
	}
	atomic.AddInt32(&f.routerCount, -1)

	//check and reset memory
	if f.routerCount <= 0 {
		atomic.StoreInt32(&f.routerCount, 0)
		f.routers = sync.Map{}
		f.routerUris = sync.Map{}
		//gc memory
		runtime.GC()
	}
	return nil
}

//get running router names
func (f *Server) GetNames() []string {
	result := make([]string, 0)
	sf := func(k, v interface{}) bool {
		name, ok := v.(string)
		if ok && name != "" {
			result = append(result, name)
		}
		return true
	}
	f.routerUris.Range(sf)
	return result
}

//get router uri by name
func (f *Server) GetUri(name string) (string, error) {
	var (
		uri string
	)
	//check
	if name == "" {
		return "", errors.New("invalid parameter")
	}

	//loop check and get uri by name
	sf := func(k, v interface{}) bool {
		tempName, ok := v.(string)
		//check and compare name
		if ok && tempName != "" && tempName == name {
			//get uri value
			uri, _ = k.(string)
			return false
		}
		return true
	}
	f.routerUris.Range(sf)
	return uri, nil
}

//get router uri name
func (f *Server) GetUriName(uri string) (string, error) {
	//check
	if uri == "" {
		return "", errors.New("invalid parameter")
	}
	v, ok := f.routerUris.Load(uri)
	if !ok || v == nil {
		return "", nil
	}
	name, subOk := v.(string)
	if !subOk || name == "" {
		return "", nil
	}
	return name, nil
}

//get router by name
func (f *Server) GetRouter(name string) (face.IRouter, error) {
	//check
	if name == "" {
		return nil, errors.New("invalid parameter")
	}
	if f.routerCount <= 0 {
		return nil, errors.New("no any active routers")
	}

	//load by name
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
	//check
	if g == nil {
		return errors.New("invalid gin engine object")
	}
	f.gin = g
	return nil
}

//start gin (optional)
//used for service mode
func (f *Server) StartGin(port int, isReleases ...bool) error {
	var (
		isRelease bool
	)
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

	//release mode check
	if isReleases != nil && len(isReleases) > 0 {
		isRelease = isReleases[0]
	}
	if isRelease {
		gin.SetMode("release")
	}

	//start server
	serverAddr := fmt.Sprintf(":%v", port)
	f.started = true
	go f.gin.Run(serverAddr)
	return nil
}

//register websocket uri, support multi instances
//methods include `GET` or `POST`
func (f *Server) RegisterUri(ur *UriRouter, methods ...string) error {
	//check
	if ur == nil || ur.RouterName == "" || ur.RouterUri == "" {
		return errors.New("invalid parameter")
	}
	if f.gin == nil {
		return errors.New("inter gin engine not init yet")
	}

	//get router name by uri
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
		CheckActiveRate: ur.CheckActiveRate,
		MaxActiveSeconds: ur.MaxActiveSeconds,
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
	router.SetCBForGenConnId(ur.CBForGenConnId)
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
	atomic.AddInt32(&f.routerCount, 1)
	return nil
}
