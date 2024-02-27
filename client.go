package tubing

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing/base"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket client
 * - persistent connect
 */

//global variable
var (
	wsClient *WebSocketClient
	wsClientOnce sync.Once
)

//message info
type WebSocketMessage struct {
	MessageType int
	Message []byte
}

//connect para info
type WebSocketConnPara struct {
	Host string
	Port int
	Uri  string //websocket root uri
	QueryPara map[string]interface{} //raw query, key -> val
	//cb func
	CBForReadMessage func(message *WebSocketMessage) error
}

//client info
type OneWSClient struct {
	connPara WebSocketConnPara
	msgType int
	u *url.URL
	conn *websocket.Conn
	connId int64
	interrupt chan os.Signal
	heartBeatChan chan struct{}
	heartBeatRate int
	writeChan chan WebSocketMessage
	doneChan chan struct{}
	closeChan chan bool
	isConnecting bool
	forceClosed bool
	autoConn bool //auto connect server switch

	//cb func
	cbForReadMessage func(message *WebSocketMessage) error
	cbForClosed func() error
	base.Util
	sync.RWMutex
}

//inter websocket manager
type WebSocketClient struct {
	messageType int
	heartBeatRate int
	autoConnect bool
	connId int64 //atomic, maybe not same with server side
	clientMap map[int64]*OneWSClient //connectId -> OneWSClient, c2s
	sync.RWMutex
}

//get single instance
func GetWSClient() *WebSocketClient {
	wsClientOnce.Do(func() {
		wsClient = NewWebSocketClient()
	})
	return wsClient
}

//construct
func NewWebSocketClient() *WebSocketClient {
	this := &WebSocketClient{
		messageType: define.MessageTypeOfOctet,
		clientMap: map[int64]*OneWSClient{},
	}
	return this
}

//set message type
func (f *WebSocketClient) SetMessageType(iType int) {
	if iType < define.MessageTypeOfJson || iType > define.MessageTypeOfOctet {
		return
	}
	//sync type
	f.messageType = iType

	//sync running clients with locker
	f.Lock()
	defer f.Unlock()
	for _, v := range f.clientMap {
		if v != nil {
			v.SetMsgType(iType)
		}
	}
}

//set auto connect switch
func (f *WebSocketClient) SetAutoConnSwitch(switcher bool) {
	f.autoConnect = switcher
	//notify all sub ws client
	f.notifyAutoConnect()
}

//set heart beat rate
func (f *WebSocketClient) SetHeartBeatRate(rate int) error {
	if rate < 0 {
		return errors.New("invalid rate parameter")
	}
	f.heartBeatRate = rate
	//notify all sub ws client
	f.notifyHeartBeatRate()
	return nil
}

//get clients
func (f *WebSocketClient) GetClients() int {
	return len(f.clientMap)
}

//create new c2s client
//client connect the target server
func (f *WebSocketClient) CreateClient(
				connPara *WebSocketConnPara,
			) (*OneWSClient, error) {
	//init new websocket client
	wsc, err := newOneWSClient(connPara, f.messageType)
	if err != nil {
		return nil, err
	}

	//gen connect id
	connId := atomic.AddInt64(&f.connId, 1)
	wsc.connId = connId

	//sync into map with locker
	f.Lock()
	defer f.Unlock()
	f.clientMap[connId] = wsc
	return wsc, nil
}

//close
func (f *WebSocketClient) Close() {
	f.Lock()
	defer f.Unlock()
	for k, v := range f.clientMap {
		if v != nil {
			v.close()
		}
		delete(f.clientMap, k)
	}
	runtime.GC()
}

//close connect
func (f *WebSocketClient) CloseConn(connectId int64) error {
	if connectId <= 0 {
		return errors.New("invalid parameter")
	}
	oneWSClient := f.getOneWSClient(connectId)
	if oneWSClient == nil {
		return errors.New("no client by id")
	}
	//close and cleanup with locker
	f.Lock()
	defer f.Unlock()
	oneWSClient.close()
	delete(f.clientMap, connectId)
	if len(f.clientMap) <= 0 {
		f.clientMap = map[int64]*OneWSClient{}
		runtime.GC()
	}
	return nil
}

//////////////////
//private func
//////////////////

//notify auto connect to all sub ws client
func (f *WebSocketClient) notifyAutoConnect() {
	if len(f.clientMap) <= 0 {
		return
	}
	//opt with locker
	f.Lock()
	defer f.Unlock()
	for _, v := range f.clientMap {
		if v != nil {
			v.SetAutoConnect(f.autoConnect)
		}
	}
}

//notify heart beat rate to all sub ws client
func (f *WebSocketClient) notifyHeartBeatRate() {
	if len(f.clientMap) <= 0 {
		return
	}
	//opt with locker
	f.Lock()
	defer f.Unlock()
	for _, v := range f.clientMap {
		if v != nil {
			v.SetHeartBeatRate(f.heartBeatRate)
		}
	}
}

//get one websocket client by session
func (f *WebSocketClient) getOneWSClient(connId int64) *OneWSClient {
	//get with locker
	f.Lock()
	defer f.Unlock()
	v, ok := f.clientMap[connId]
	if !ok || v == nil {
		return nil
	}
	return v
}

//////////////////////
//api for oneWSClient
/////////////////////

func newOneWSClient(
			connPara *WebSocketConnPara,
			msgType int,
		) (*OneWSClient, error) {
	//self init
	this := &OneWSClient{
		connPara: *connPara,
		msgType: msgType,
		autoConn: true, //auto connect default
		interrupt: make(chan os.Signal, 1),
		heartBeatChan: make(chan struct{}, 1),
		writeChan: make(chan WebSocketMessage, define.DefaultChanSize),
		doneChan: make(chan struct{}, 1),
		closeChan: make(chan bool, 1),
		cbForReadMessage: connPara.CBForReadMessage,
	}
	//dial server
	err := this.dialServer()
	return this, err
}

//check connect
func (f *OneWSClient) IsClosed() bool {
	if f.forceClosed {
		return true
	}
	return f.conn == nil
}

//get conn id
func (f *OneWSClient) GetConnId() int64 {
	return f.connId
}

//set msg type
func (f *OneWSClient) SetMsgType(iType int) {
	if iType < define.MessageTypeOfJson ||
		iType > define.MessageTypeOfOctet {
		return
	}
	f.msgType = iType
}

//set auto connect switcher
func (f *OneWSClient) SetAutoConnect(switcher bool) {
	f.autoConn = switcher
	if switcher && !f.forceClosed && f.conn == nil {
		//force dail server
		f.dialServer()
	}
}

//set heart beat rate
func (f *OneWSClient) SetHeartBeatRate(rate int) error {
	//check
	if rate < 0 {
		return errors.New("invalid rate parameter")
	}
	//setup and notify
	f.heartBeatRate = rate
	if rate > 0 {
		f.heartBeatChan <- struct{}{}
	}
	return nil
}

//send message data
func (f *OneWSClient) SendMessage(message[]byte) error {
	var (
		m any = nil
	)
	//check
	if message == nil {
		return errors.New("invalid parameter")
	}
	if f.writeChan == nil || len(f.writeChan) >= define.DefaultChanSize {
		return errors.New("write chan is nil or full")
	}
	if f.forceClosed {
		return errors.New("client has force closed")
	}
	if f.conn == nil {
		if !f.isConnecting {
			f.autoConnect()
		}
		return errors.New("connect has closed")
	}

	//defer
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("WebSocketClient:SendData panic, err:%v, track:%v\n", subErr, string(debug.Stack()))
		}
	}()

	//init request
	req := WebSocketMessage{
		MessageType: f.msgType,
		Message: message,
	}

	//async send to chan
	select {
	case f.writeChan <- req:
	}
	return nil
}

//detail server
func (f *OneWSClient) DialServer() error {
	return f.dialServer()
}

///////////////
//private func
///////////////

//close
func (f *OneWSClient) close() {
	var (
		m any = nil
	)

	//check
	if f.conn == nil || f.forceClosed {
		return
	}

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("WebSocketClient:Close panic, err:%v", err)
		}
	}()

	//send close with locker
	f.Lock()
	defer f.Unlock()
	f.forceClosed = true
	select {
	case f.closeChan <- true:
	}
}

//auto connect
func (f *OneWSClient) autoConnect() {
	//check
	if !f.autoConn {
		//not allow auto connect server
		return
	}

	//delay connect server
	sf := func() {
		f.dialServer()
	}
	time.AfterFunc(time.Second * 2, sf)
}

//dial server
func (f *OneWSClient) dialServer() error {
	var (
		m any = nil
	)

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Println("OneWSClient:dialServer panic, err:", err)
			//notify interrupt
			signal.Notify(f.interrupt, os.Interrupt)
		}
	}()

	if f.conn != nil {
		return errors.New("client conn not nil")
	}
	if f.forceClosed {
		return errors.New("client has force closed")
	}
	if f.isConnecting {
		return errors.New("client is connection server")
	}
	f.isConnecting = true

	//check and init url
	if f.u == nil {
		//init url
		f.u = &url.URL{
			Scheme: "ws",
			Host: fmt.Sprintf("%s:%d", f.connPara.Host, f.connPara.Port),
			Path: f.connPara.Uri,
		}
		//init query
		q := f.u.Query()
		if f.connPara.QueryPara != nil {
			for k, v := range f.connPara.QueryPara {
				q.Set(k, fmt.Sprintf("%v", v))
			}
		}
		f.u.RawQuery = q.Encode()
	}

	//try dial server with locker
	conn, _, err := websocket.DefaultDialer.Dial(f.u.String(), nil)
	if err != nil {
		f.isConnecting = false
		return err
	}

	//sync connect
	f.conn = conn

	//spawn son process
	go f.readMessageFromServer()
	go f.runMainProcess()
	f.isConnecting = false
	return nil
}

//heart beat
func (f *OneWSClient) heartBeat() error {
	if f.conn == nil {
		return errors.New("conn is nil")
	}
	err := f.SendMessage([]byte(define.MessageBodyOfHeartBeat))
	return err
}

//son process for send and receive
func (f *OneWSClient) runMainProcess() {
	var (
		//heartTicker = time.NewTicker(time.Second * define.ClientHeartBeatRate)
		writeMessage WebSocketMessage
		isOk                      bool
		err                       error
		m any = nil
	)

	//defer
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("WebSocketClient:runMainProcess panic, err:%v", subErr)
		}
		//stop and close
		f.Lock()
		defer f.Unlock()
		//heartTicker.Stop()
		if f.conn != nil {
			f.conn.Close()
			f.conn = nil
		}
		f.isConnecting = false
	}()

	//setup ticker func
	heatBeatTicker := func() {
		sf := func() {
			if f.heartBeatRate > 0 && f.heartBeatChan != nil {
				f.heartBeatChan <- struct{}{}
			}
		}
		if f.heartBeatRate > 0 {
			duration := time.Duration(f.heartBeatRate) * time.Second
			time.AfterFunc(duration, sf)
		}
	}

	//main loop
	//log.Printf("WebSocketClient:runMainProcess start\n")
	for {
		select {
		case writeMessage, isOk = <- f.writeChan:
			if isOk {
				//write message
				err = f.conn.WriteMessage(f.msgType, writeMessage.Message)
				if err != nil {
					log.Printf("WebSocketClient:runMainProcess write failed, err:%v", err)
					if err == io.EOF {
						return
					}
					if errors.Is(err, syscall.EPIPE) {
						log.Printf("WebSocketClient:runMainProcess write failed, broken pipe error\n")
						f.autoConnect()
						return
					}
				}
			}
		case <- f.closeChan:
			{
				return
			}
		case <- f.interrupt:
			{
				err = f.conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(
						websocket.CloseNormalClosure,
						"",
					),
				)
				if err != nil {
					log.Printf("WebSocketClient:runMainProcess write closed, err:%v", err)
					return
				}
			}
		case <- f.heartBeatChan:
			{
				//heart beat
				f.heartBeat()

				//send next ticker
				heatBeatTicker()
			}
		}
	}
}

//read message from remote server
func (f *OneWSClient) readMessageFromServer() {
	var (
		messageType int
		message []byte
		err error
		m any = nil
	)

	//defer
	defer func() {
		if subErr := recover(); subErr != m {
			log.Printf("WebSocketClient:readMessageFromServer panic, err:%v", subErr)
		}
	}()

	//loop receive message
	//log.Printf("WebSocketClient:readMessageFromServer start\n")
	for {
		//check
		if f.conn == nil {
			log.Printf("WebSocketClient:readMessageFromServer connect is nil\n")
			return
		}

		//read original message
		messageType, message, err = f.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocketClient:readMessageFromServer failed, err:%v", err.Error())
			if errors.Is(err, syscall.EPIPE) {
				log.Printf("WebSocketClient:readMessageFromServer write failed, broken pipe error\n")
				f.autoConnect()
				return
			}
			if err == io.EOF {
				return
			}
		}

		//call cb for read message
		if f.cbForReadMessage != nil {
			//packet data and send to chan
			wsMsg := WebSocketMessage{
				MessageType: messageType,
				Message: message,
			}
			f.cbForReadMessage(&wsMsg)
		}
	}
}