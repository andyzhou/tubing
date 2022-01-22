package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"tubing/define"
	"tubing/lib/client"
)

/*
 * dynamic router
 */

const (
	ToDomain = "http://127.0.0.1:8687"
)

//face info
type Router struct {
	to string
	requester IRequester
	client IClient
	websocket IWebsocket //parent reference
}

//construct
func NewRouter() *Router {
	this := &Router{
		requester: NewRequester(),
		to: ToDomain,
	}
	return this
}

//set websocket reference
func (f *Router) SetWebSocket(ws IWebsocket) bool {
	if ws == nil {
		return false
	}
	f.websocket = ws
	return true
}

//cb for router
func (f *Router) Run(c *gin.Context) {
	var (
		requestUrl string
		reqContentType string
	)

	//get key data
	anyPath := c.Param(define.AnyPath)
	reqContentType = c.Request.Header.Get(define.HeaderOfContentType)
	requestUrl = f.requester.GetRequestUrl(c)
	log.Printf("Router:Run, anyPath:%v, contentType:%v, requestUrl:%v",
				anyPath, reqContentType, requestUrl)

	//check request uri is websocket channel
	if f.isWebSocketUri(anyPath) {
		if f.websocket == nil {
			return
		}
		f.websocket.ProcessConn(c)
		return
	}

	//set real to url
	anyPath = strings.TrimLeft(anyPath, "/")
	to := strings.TrimRight(f.to, "/")
	to = fmt.Sprintf("%v/%v", to, anyPath)

	//read original request data
	originData, err := c.GetRawData()
	if err != nil {
		log.Printf("Router:Run, read raw data failed, err:%v", err.Error())
		return
	}

	//setup content type
	contentType := define.MessageTypeOfOctet
	if c.ContentType() == define.ContentTypeOfJson {
		contentType = define.MessageTypeOfJson
	}
	log.Println("org:", originData, ", type:", contentType)

	//init http client
	cli := client.GetHttpClient()
	httpReq := &client.HttpReq{
		Method: f.requester.GetReqMethod(c),
		ContentType: reqContentType,
		To: to,
		Query: requestUrl,
		RawData: originData,
	}

	//send client request to target
	body, err := cli.SendRequest(httpReq)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, body)
}

//check request uri is websocket channel
func (f *Router) isWebSocketUri(path string) bool {
	return strings.HasPrefix(path, define.WebSocketRoot)
}