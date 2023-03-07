package client

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing/define"
	"github.com/valyala/fasthttp"
	"log"
	"sync"
	"time"
)

/*
 * http client
 * - one single instance mode
 * - process request in queue
 * - multi inter clients for performance
 */

//global variable
var (
	httpClient *HttpClient
	httpClientOnce sync.Once
)

//face info
type (
	//request info
	HttpReq struct {
		Method string
		ContentType string
		To string
		Query string
		RawData []byte
		IsAsync bool
	}

	//response info
	httpResp struct {
		body string
		err error
	}

	//inter request info
	interHttpReq struct {
		orgReq HttpReq
		resp chan httpResp
	}
)

//client info
type HttpClient struct {
	TimeOut int
	//private property
	client *fasthttp.Client
	timeout time.Duration
	reqChan chan interHttpReq
	closeChan chan struct{}
}

//get single instance
func GetHttpClient() *HttpClient {
	httpClientOnce.Do(func() {
		httpClient = NewHttpClient()
	})
	return httpClient
}

//construct
func NewHttpClient() *HttpClient {
	this := &HttpClient{
		TimeOut: define.ReqTimeOutSeconds,
		reqChan: make(chan interHttpReq, define.DefaultChanSize),
		closeChan: make(chan struct{}, 1),
	}
	this.interInit()
	go this.runMainProcess()
	return this
}

//quit
func (f *HttpClient) Quit() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("HttpClient:Quit panic, err:%v", err)
		}
	}()
	select {
	case f.closeChan <- struct{}{}:
	}
}

//send http request to target server
func (f *HttpClient) SendRequest(req *HttpReq) (string, error)  {
	var (
		body string
	)
	//check
	if req == nil {
		return body, errors.New("invalid parameter")
	}
	//init inter request
	interReq := interHttpReq{
		orgReq: *req,
		resp: make(chan httpResp, 1),
	}

	//async mode
	if req.IsAsync {
		select {
		case f.reqChan <- interReq:
		}
		return body, nil
	}

	//sync mode
	f.reqChan <- interReq

	//wait for response
	resp, ok := <- interReq.resp
	if !ok {
		return body, errors.New("no any response")
	}
	return resp.body, resp.err
}

///////////////
//private func
///////////////

//run main process
func (f *HttpClient) runMainProcess() {
	var (
		req interHttpReq
		isOk bool
	)
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("HttpClient:runMainProcess panic, err:%v", err)
		}
		close(f.reqChan)
		close(f.closeChan)
	}()

	//loop
	for {
		select {
		case req, isOk = <- f.reqChan:
			if isOk {
				f.sendRealHttpReq(&req)
			}
		case <- f.closeChan:
			return
		}
	}
}

//send real http request
func (f *HttpClient) sendRealHttpReq(interReq *interHttpReq) {
	//init request and response
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()

	//defer
	defer func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
	}()

	//get org request
	orgReq := interReq.orgReq

	//setup resp
	resp := httpResp{}

	//set real request uri
	uri := fmt.Sprintf("%v?%v", orgReq.To, orgReq.Query)

	//set header
	if orgReq.ContentType != "" {
		req.Header.SetContentType(orgReq.ContentType)
	}

	//set method and request
	req.Header.SetMethod(orgReq.Method)
	req.SetRequestURI(uri)
	if orgReq.RawData != nil && len(orgReq.RawData) > 0 {
		req.SetBody(orgReq.RawData)
	}

	//send request
	err := f.client.DoTimeout(req, res, f.timeout)
	if err != nil {
		if !orgReq.IsAsync {
			//sync mode, need send response
			resp.err = err
			interReq.resp <- resp
		}
		return
	}

	if !orgReq.IsAsync {
		//sync mode, need send response
		//read body
		body := string(res.Body())
		resp.body = body
		interReq.resp <- resp
	}
}

//inter init
func (f *HttpClient) interInit() {
	//init http client
	f.timeout = time.Duration(f.TimeOut) * time.Second
	cli := &fasthttp.Client{
		MaxConnsPerHost: define.ReqMaxConn,
		ReadTimeout: f.timeout,
	}
	f.client = cli
}