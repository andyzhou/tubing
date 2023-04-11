package tubing

import (
	"bytes"
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
	SessionName string
	CBForConnected func(session string, para map[string]interface{}) error
	CBForClosed func(session string) error
	CBForRead func(session string, messageType int, message []byte) error
}

//face info
type Server struct {
	gin     *gin.Engine
	rootUri string //like /<url>/[:module]
	router face.IRouter //router interface
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
	}
	return this
}

//get coder
func (f *Server) GetCoder() face.ICoder {
	if f.router != nil {
		return f.router.GetCoder()
	}
	return nil
}

//get pattern para
func (f *Server) GetPatternPara(name string) string {
	if f.router == nil {
		return ""
	}
	return f.router.GetPatternPara(name)
}

//set gin
func (f *Server) SetGin(g *gin.Engine) error {
	if g == nil {
		return errors.New("invalid gin engine object")
	}
	f.gin = g
	return nil
}

//set root uri, STEP-1
func (f *Server) SetRootUriPattern(
			rootUri string,
			patternNames ...string) error {
	//check
	if rootUri == "" {
		return errors.New("invalid parameter")
	}
	//set root uri pattern
	if patternNames != nil {
		bf := bytes.NewBuffer(nil)
		bf.WriteString(rootUri)
		for _, patternName := range patternNames {
			tmp := fmt.Sprintf("/{:%s}", patternName)
			bf.WriteString(tmp)
		}
		f.rootUri = bf.String()
	}else{
		f.rootUri = rootUri
	}
	return nil
}

//register websocket uri
//methods include `GET` or `POST`
func (f *Server) RegisterUri(ur *UriRouter, methods ...string) error {
	//check
	if ur == nil || ur.SessionName == "" {
		return errors.New("invalid parameter")
	}
	if f.gin == nil {
		return errors.New("inter gin engine not init yet")
	}
	if f.rootUri == "" {
		return errors.New("root uri not setup")
	}
	if f.router != nil {
		return errors.New("router had registered")
	}

	//setup method
	method := ""
	if methods != nil && len(methods) > 0 {
		method = methods[0]
	}

	//init new router
	router := face.NewRouter()
	//setup relate key data and callbacks
	router.SetSessionName(ur.SessionName)
	router.SetCBForConnected(ur.CBForConnected)
	router.SetCBForClosed(ur.CBForClosed)
	router.SetCBForRead(ur.CBForRead)

	//begin register
	switch method {
	case define.ReqMethodOfPost:
		f.gin.POST(f.rootUri, router.Entry)
	case define.ReqMethodOfGet:
		f.gin.GET(f.rootUri, router.Entry)
	default:
		f.gin.Any(f.rootUri, router.Entry)
	}

	//sync inter router
	f.router = router
	return nil
}

//close
func (f *Server) Close() error {
	//check
	if f.router == nil {
		return errors.New("router hasn't registered")
	}
	//try close all
	f.router.GetManager().Close()
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

//get conn by session
func (f *Server) GetConn(session string) (face.IWSConn, error) {
	//check
	if session == "" {
		return nil, errors.New("invalid parameter")
	}
	if f.router == nil {
		return nil, errors.New("router hasn't registered")
	}
	//try get websocket connect
	conn, err := f.router.GetManager().GetConn(session)
	return conn, err
}

//close sessions
func (f *Server) CloseSession(sessions ...string) error {
	//check
	if sessions == nil || len(sessions) <= 0 {
		return errors.New("invalid parameter")
	}
	if f.router == nil {
		return errors.New("router hasn't registered")
	}
	//close session
	err := f.router.GetManager().CloseConn(sessions...)
	return err
}

//send message to sessions
func (f *Server) SendMessage(messageType int, message []byte, sessions ... string) error {
	//check
	if messageType < 0 || message == nil || sessions == nil {
		return errors.New("invalid parameter")
	}
	if f.router == nil {
		return errors.New("router hasn't registered")
	}
	//send message
	err := f.router.GetManager().SendMessage(messageType, message, sessions...)
	return err
}

//cast message
func (f *Server) CastMessage(messageType int, message []byte, tags ... string) error {
	//check
	if messageType < 0 || message == nil {
		return errors.New("invalid parameter")
	}
	if f.router == nil {
		return errors.New("router hasn't registered")
	}
	//cast message
	err := f.router.GetManager().CastMessage(messageType, message)
	return err
}