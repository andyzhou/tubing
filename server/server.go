package server

import (
	"tubing/lib/cmd"
)

/*
 * inter server face
 */

//server info
type Server struct {
	httpServer *HttpServer
}

//construct
func NewServer(appCfg *cmd.AppEnvConfig) *Server {
	this := &Server{
		httpServer: NewHttpServer(appCfg.HttpPort),
	}
	return this
}

//start
func (s *Server) Start() {
	s.httpServer.Start()
}