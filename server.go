package tubing

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"sync"
	"tubing/websocket"
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
	gin *gin.Engine
	rootUri string //root websocket uri
	router websocket.IRouter //router interface
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
	this := &Server{
		gin: gin.Default(),
	}
	if gs != nil && len(gs) > 0 {
		this.gin = gs[0]
	}
	return this
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
func (f *Server) SetRootUri(uri string) error {
	//check
	if uri == "" {
		return errors.New("invalid parameter")
	}
	f.rootUri = uri
	return nil
}

//register websocket uri, STEP-2
func (f *Server) RegisterUri(ur *UriRouter) error {
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

	//init new router
	router := websocket.NewRouter()
	router.SetSessionName(ur.SessionName)
	router.SetCBForConnected(ur.CBForConnected)
	router.SetCBForClosed(ur.CBForClosed)
	router.SetCBForRead(ur.CBForRead)

	//begin register
	f.gin.GET(f.rootUri, router.Entry)

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
	//start server
	serverAddr := fmt.Sprintf(":%v", port)
	go f.gin.Run(serverAddr)
	return nil
}

//get conn by session
func (f *Server) GetConn(session string) (websocket.IWSConn, error) {
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