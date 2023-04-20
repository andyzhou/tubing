package tubing

import (
	"errors"
	"fmt"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
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
	readChan chan WebSocketMessage
	writeChan chan WebSocketMessage
	doneChan chan struct{}
	closeChan chan bool
	hasClosed bool

	//cb func
	cbForReadMessage func(message *WebSocketMessage) error
	cbForClosed func() error
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

//close connect
func (f *WebSocketClient) Close(connId int64) error {
	if connId <= 0 {
		return errors.New("invalid parameter")
	}
	oneWSClient := f.getOneWSClient(connId)
	if oneWSClient == nil {
		return errors.New("no client by id")
	}
	oneWSClient.close()
	f.clientMap.Delete(connId)
	atomic.AddInt64(&f.clients, -1)
	if f.clients < 0 {
		atomic.StoreInt64(&f.clients, 0)
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
		readChan: make(chan WebSocketMessage, define.DefaultChanSize),
		writeChan: make(chan WebSocketMessage, define.DefaultChanSize),
		doneChan: make(chan struct{}),
		closeChan: make(chan bool, 1),
		cbForReadMessage: connPara.CBForReadMessage,
	}
	//dial server
	err := this.dialServer()
	return this, err
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
	//check
	if message == nil {
		return errors.New("invalid parameter")
	}

	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketClient:SendData panic, err:%v", err)
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

///////////////
//private func
///////////////

//close
func (f *OneWSClient) close() {
	if f.conn == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketClient:Close panic, err:%v", err)
		}
	}()
	select {
	case f.closeChan <- true:
	}
}

//dial server
func (f *OneWSClient) dialServer() error {
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Println("OneWSClient:dialServer panic, err:", err)
			//init interrupt
			signal.Notify(f.interrupt, os.Interrupt)
		}
	}()

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

	//try dial server
	conn, _, err := websocket.DefaultDialer.Dial(f.u.String(), nil)
	if err != nil {
		return err
	}

	//sync object
	f.conn = conn

	//spawn son process
	go f.readMessageFromServer(f.doneChan)
	go f.runMainProcess()
	return nil
}

//son process for send and receive
func (f *OneWSClient) runMainProcess() {
	var (
		heartTicker = time.NewTicker(time.Second * define.ClientHeartBeatRate)
		readMessage, writeMessage WebSocketMessage
		isOk                      bool
		err                       error
	)
	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketClient:runMainProcess panic, err:%v", err)
		}
		f.hasClosed = true
		heartTicker.Stop()
		close(f.writeChan)
		close(f.closeChan)
	}()

	//loop
	for {
		select {
		case readMessage, isOk = <- f.readChan:
			if isOk {
				//read original message
				if f.cbForReadMessage != nil {
					f.cbForReadMessage(&readMessage)
				}
			}
		case writeMessage, isOk = <- f.writeChan:
			if isOk {
				//write message
				err = f.conn.WriteMessage(f.msgType, writeMessage.Message)
				if err != nil {
					if err == io.EOF {
						return
					}
					log.Printf("WebSocketClient:runMainProcess write failed, err:%v", err)
				}
			}
		case <- f.doneChan:
			return
		case <- f.closeChan:
			return
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
				select {
				case <- f.doneChan:
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

//heart beat
func (f *OneWSClient) heartBeat() error {
	if f.conn == nil {
		return errors.New("conn is nil")
	}
	err := f.SendMessage([]byte(define.MessageBodyOfHeartBeat))
	return err
}

//read message from remote server
func (f *OneWSClient) readMessageFromServer(done chan struct{}) {
	var (
		messageType int
		message []byte
		err error
	)

	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WebSocketClient:readMessageFromServer panic, err:%v", err)
		}
		close(f.readChan)
		close(done)
	}()

	//loop receive message
	for {
		if f.hasClosed {
			return
		}
		//read original message
		messageType, message, err = f.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocketClient:readMessageFromServer failed, err:%v", err.Error())
			return
		}
		//packet data and send to chan
		wsMsg := WebSocketMessage{
			MessageType: messageType,
			Message: message,
		}
		f.readChan <- wsMsg
	}
}