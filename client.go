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
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

/*
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
	id int64
	u *url.URL
	conn *websocket.Conn
	interrupt chan os.Signal
	writeChan chan WebSocketMessage
	doneChan chan struct{}
	closeChan chan bool
	isConnecting bool
	forceClosed bool

	//cb func
	cbForReadMessage func(message *WebSocketMessage) error
	cbForClosed func() error
	base.Util
	sync.RWMutex
}

//inter websocket manager
type WebSocketClient struct {
	messageType int
	clientId int64
	clients int64
	clientMap sync.Map //clientId -> OneWSClient, c2s
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
		clientMap: sync.Map{},
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

	//sync running clients
	sf := func(k, v interface{}) bool{
		wsc, ok := v.(*OneWSClient)
		if ok && wsc != nil {
			wsc.SetMsgType(iType)
		}
		return true
	}
	f.clientMap.Range(sf)
}

//get clients
func (f *WebSocketClient) GetClients() int64 {
	return f.clients
}

//create new c2s client
//client connect the target server
func (f *WebSocketClient) CreateClient(
				connPara *WebSocketConnPara,
			) (*OneWSClient, error) {
	//gen new client id
	newClientId := atomic.AddInt64(&f.clientId, 1)

	//init new websocket client
	wsc, err := newOneWSClient(connPara, f.messageType)
	if err != nil {
		return nil, err
	}

	//set key data
	wsc.id = newClientId

	//sync into map
	f.clientMap.Store(newClientId, wsc)
	atomic.AddInt64(&f.clients, 1)
	return wsc, nil
}

//close
func (f *WebSocketClient) Close() {
	if f.clients <= 0 {
		return
	}
	sf := func(k, v interface {}) bool {
		clientId, _ := k.(int64)
		wsc, _ := v.(*OneWSClient)
		if clientId > 0 && wsc != nil {
			wsc.close()
		}
		f.clientMap.Delete(clientId)
		return true
	}
	f.clientMap.Range(sf)
	atomic.StoreInt64(&f.clients, 0)
}

//close connect
func (f *WebSocketClient) CloseConn(connId int64) error {
	if connId <= 0 {
		return errors.New("invalid parameter")
	}
	oneWSClient := f.getOneWSClient(connId)
	if oneWSClient == nil {
		return errors.New("no client by id")
	}
	//close and cleanup
	oneWSClient.close()
	f.clientMap.Delete(connId)
	atomic.AddInt64(&f.clients, -1)
	if f.clients < 0 {
		atomic.StoreInt64(&f.clients, 0)
		f.clientMap = sync.Map{}
	}
	return nil
}

//////////////////
//private func
//////////////////

//get one websocket client by session
func (f *WebSocketClient) getOneWSClient(connId int64) *OneWSClient {
	v, ok := f.clientMap.Load(connId)
	if !ok || v == nil {
		return nil
	}
	conn, ok := v.(*OneWSClient)
	if !ok {
		return nil
	}
	return conn
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
		interrupt: make(chan os.Signal, 1),
		//readChan: make(chan WebSocketMessage, define.DefaultChanSize),
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
	return f.id
}

//set msg type
func (f *OneWSClient) SetMsgType(iType int) {
	if iType < define.MessageTypeOfJson ||
		iType > define.MessageTypeOfOctet {
		return
	}
	f.msgType = iType
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

	f.Lock()
	defer f.Unlock()
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
		heartTicker = time.NewTicker(time.Second * define.ClientHeartBeatRate)
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
		heartTicker.Stop()
		if f.conn != nil {
			f.conn.Close()
			f.conn = nil
		}
		f.isConnecting = false
	}()

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
		case <- heartTicker.C:
			{
				//heart beat
				f.heartBeat()
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