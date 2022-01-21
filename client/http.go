package client

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"strings"
	"time"
	"tubing/define"
)

/*
 * http client
 */

//client info
type HttpClient struct {
	Method string
	ContentType string
	To string
	Query string
	RawData []byte
	TimeOut int
}

//construct
func NewHttpClient() *HttpClient {
	this := &HttpClient{
		TimeOut: define.ReqTimeOutSeconds,
	}
	return this
}

//send http request to target server
func (f *HttpClient) Send() (string, error)  {
	var (
		body string
		err error
	)
	method := strings.ToUpper(f.Method)
	switch method {
	case define.ReqMethodOfGet:
		body, err = f.get()
	case define.ReqMethodOfPost:
		body, err = f.post()
	default:
		err = fmt.Errorf("invalid method of %v", method)
	}
	return body, err
}


///////////////
//private func
///////////////

//for get method
func (f *HttpClient) get() (string, error) {
	return f.sendClientReq(define.ReqMethodOfGet)
}

//for post method
func (f *HttpClient) post() (string, error) {
	return f.sendClientReq(define.ReqMethodOfPost)
}

//build http client and send request
func (f *HttpClient) sendClientReq(method string) (string, error) {
	//init request and response
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	//init http client
	timeoutDur := time.Duration(f.TimeOut) * time.Second
	cli := &fasthttp.Client{
		MaxConnsPerHost: define.ReqMaxConn,
		ReadTimeout: timeoutDur,
	}
	//defer
	defer func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
	}()
	//set real request uri
	uri := fmt.Sprintf("%v?%v", f.To, f.Query)
	//set header
	if f.ContentType != "" {
		req.Header.SetContentType(f.ContentType)
	}
	//set method and request
	req.Header.SetMethod(method)
	req.SetRequestURI(uri)
	if f.RawData != nil && len(f.RawData) > 0 {
		req.SetBody(f.RawData)
	}
	//send request
	err := cli.DoTimeout(req, res, timeoutDur)
	if err != nil {
		return "", err
	}
	//read body
	body := string(res.Body())
	return body, nil
}