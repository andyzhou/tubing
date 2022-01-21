package http

import "github.com/gin-gonic/gin"

/*
 * http relate interface define
 */

//interface of router
type IRouter interface {
	Run(c *gin.Context)
	SetWebSocket(ws IWebsocket) bool
}

//interface of websocket
type IWebsocket interface {
	Init(gin *gin.Engine, rootUri ...string)
	ProcessConn(c *gin.Context)
	GetRootUri() string
}

//interface of requester
type IRequester interface {
	GetRequestUri(c *gin.Context) string
	GetRequestUrl(c *gin.Context) string
	GetReqMethod(c *gin.Context) string
}

//interface of http client
type IClient interface {
	Send() (string, error)
}