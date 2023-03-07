package http

import (
	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"net/url"
)

/*
 * request processor
 */

//face info
type Requester struct {
}

//construct
func NewRequester() *Requester {
	this := &Requester{
	}
	return this
}

//get original request uri
func (f *Requester) GetRequestUri(c *gin.Context) string {
	c.Request.ParseForm()
	u, _ := url.Parse(c.Request.RequestURI)
	return u.Path
}

//get original request url
func (f *Requester) GetRequestUrl(c *gin.Context) string {
	var (
		query string
		method string
	)
	method = c.Request.Method
	switch method {
	case define.ReqMethodOfGet:
		query = c.Request.URL.RawQuery
	case define.ReqMethodOfPost:
		c.Request.ParseForm()
		param := c.Request.PostForm
		if len(param) > 0 {
			query = param.Encode()
		}
	}
	return query
}

//get origin request method
func (f *Requester) GetReqMethod(c *gin.Context) string {
	return c.Request.Method
}