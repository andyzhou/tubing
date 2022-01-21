package server

import (
	"tubing/cmd"
)

/*
 * inter server face
 */

//server info
type Server struct {
	httpServer *HttpServer
}

//construct
func NewServer(appCfg *cmd.AppConfig) *Server {
	this := &Server{
		httpServer: NewHttpServer(appCfg.HttpPort),
	}
	return this
}

//start
func (s *Server) Start() {
	s.httpServer.Start()
}