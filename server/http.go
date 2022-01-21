package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"tubing/define"
	th "tubing/http"
)

/*
 * http server
 */

//server info
type HttpServer struct {
	port int
	server *gin.Engine
	router th.IRouter
	websocket th.IWebsocket
}

//construct
func NewHttpServer(port int) *HttpServer {
	this := &HttpServer{
		port: port,
		router: th.NewRouter(),
		websocket: th.NewWebSocket(),
	}
	this.interInit()
	return this
}

//start
func (s *HttpServer) Start() {
	addr := fmt.Sprintf(":%v", s.port)
	s.server.Run(addr)
}

//get gin instance
func (s *HttpServer) GetGinObj() *gin.Engine {
	return s.server
}

////////////////
//private func
////////////////

//inter init
func (s *HttpServer) interInit() {
	//init gin
	gin := gin.Default()
	s.server = gin

	//init websocket
	s.websocket.Init(gin)

	//set ws reference for http
	s.router.SetWebSocket(s.websocket)

	//dynamic route
	anyPath := fmt.Sprintf("/*%v", define.AnyPath)
	s.server.Any(anyPath, s.router.Run)
}