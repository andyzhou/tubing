package tubing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/andyzhou/tinylib/queue"
	"github.com/andyzhou/tubing/base"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket client
 * - persistent connect
 */

//global variable
var (
	wsClient     *WebSocketClient
	wsClientOnce sync.Once
)

//message info
type WebSocketMessage struct {
	MessageType int
	Message     []byte
}

//connect para info
type WebSocketConnPara struct {
	Host        string
	Port        int
	Uri         string                 //websocket root uri
	ReadTimeout float64                // xx seconds
	QueryPara   map[string]interface{} //raw query, key -> val
	//cb func
	CBForReadMessage func(message *WebSocketMessage) error
}

//client info
type OneWSClient struct {
	connPara      WebSocketConnPara
	msgType       int
	u             *url.URL
	conn          *websocket.Conn
	connId        int64
	interrupt     chan os.Signal
	heartBeatChan chan struct{}
	heartBeatRate int
	writeChan     chan WebSocketMessage
	doneChan      chan struct{}
	closeChan     chan bool
	isConnecting  bool
	forceClosed   bool
	autoConn      bool //auto connect server switch
	connLocker    sync.RWMutex

	//cb func
	cbForReadMessage func(message *WebSocketMessage) error
	cbForClosed      func() error
	base.Util
	sync.RWMutex
}

//inter websocket manager
type WebSocketClient struct {
	messageType    int
	heartBeatRate  int
	autoConnect    bool
	connId         int64                  //atomic, maybe not same with server side
	clientMap      map[int64]*OneWSClient //connectId -> OneWSClient, c2s
	consumerTicker *queue.Ticker

	//cb setup
	cbForReadMessage func(int64, int, []byte) error
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
//consumeRates used for consume ticker rate
func NewWebSocketClient(consumeRates ...float64) *WebSocketClient {
	var (
		consumeRate float64
	)

	//detect consume rate
	if consumeRates != nil && len(consumeRates) > 0 {
		consumeRate = consumeRates[0]
	}
	if consumeRate <= 0 {
		consumeRate = define.DefaultConnectTicker
	}

	//self init
	this := &WebSocketClient{
		messageType: define.MessageTypeOfOctet,
		clientMap: map[int64]*OneWSClient{},
		consumerTicker: queue.NewTicker(consumeRate),
	}
	this.interInit()
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

//relate cb setup
//cb for read server message
func (f *WebSocketClient) SetCBForReadMessage(cb func(int64, int, []byte) error) {
	f.cbForReadMessage = cb
}

//get clients
func (f *WebSocketClient) GetClients() int {
	return len(f.clientMap)
}

//create new c2s client
//client connect the target server
func (f *WebSocketClient) CreateClient(connPara *WebSocketConnPara) (*OneWSClient, error) {
	//init new websocket client
	wsc := newOneWSClient(connPara, f.messageType)

	//dial server
	err := wsc.dialServer()
	if err != nil {
		return wsc, nil
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

//cb for new client dial ticker
func (f *WebSocketClient) cbForConsumerOpt() error {
	var (
		messageType int
		message []byte
		err error
	)
	//check
	if len(f.clientMap) <= 0 {
		return errors.New("no any client connect")
	}

	//loop check with locker
	for connId, ows := range f.clientMap {
		if ows == nil {
			continue
		}
		//read message from server
		messageType, message, err = ows.readServerMessage()
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				//reconnect client
				ows.autoConnect()
			}
			continue
		}
		if messageType >= 0 && f.cbForReadMessage != nil {
			f.cbForReadMessage(connId, messageType, message)
		}
	}
	return nil
}

//inter init
func (f *WebSocketClient) interInit() {
	f.consumerTicker.SetCheckerCallback(f.cbForConsumerOpt)
}

//////////////////////
//api for oneWSClient
/////////////////////

func newOneWSClient(connPara *WebSocketConnPara, msgType int) *OneWSClient {
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
	return this
}

//quit
func (f *OneWSClient) Quit() {
	f.close()
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
	//check
	if message == nil {
		return errors.New("invalid parameter")
	}
	if f.forceClosed {
		return errors.New("client has force closed")
	}

	//opt with locker
	f.connLocker.Lock()
	defer f.connLocker.Unlock()
	if f.conn == nil {
		if !f.isConnecting {
			f.autoConnect()
		}
		return errors.New("connect has closed")
	}
	err := f.conn.WriteMessage(f.msgType, message)
	return err
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
	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}
	f.forceClosed = true
}

//auto connect
func (f *OneWSClient) autoConnect() {
	//check
	if !f.autoConn {
		//not allow auto connect server
		return
	}

	//delay connect server
	f.dialServer()
	time.Sleep(time.Second/10)
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

	//try dial server with timeout
	timeOut := time.Duration(define.DefaultDialTimeOut) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()

	//dial server
	f.isConnecting = true
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, f.u.String(), nil)
	if err != nil {
		f.isConnecting = false
		return err
	}

	//sync connect
	f.connLocker.Lock()
	defer f.connLocker.Unlock()
	f.conn = conn
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

//read message from server side
func (f *OneWSClient) readServerMessage() (int, []byte, error) {
	//check
	if f.conn == nil || f.forceClosed {
		return 0, nil, errors.New("connect is closed")
	}

	//setup read timeout
	readTimeout := f.connPara.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = define.DefaultReadTimeout
	}

	//read original message
	f.conn.SetReadDeadline(time.Now().Add(time.Duration(readTimeout * float64(time.Second))))
	messageType, message, err := f.conn.ReadMessage()
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			//auto connect
			f.autoConnect()
			return 0, nil, err
		}
	}

	//call cb for read message
	if messageType >= 0 && f.cbForReadMessage != nil {
		//packet data and send to chan
		wsMsg := WebSocketMessage{
			MessageType: messageType,
			Message: message,
		}
		f.cbForReadMessage(&wsMsg)
	}

	return messageType, message, nil
}

////write message to server side
//func (f *OneWSClient) writeServerMessage() error {
//	//check
//	if f.conn == nil || f.forceClosed {
//		return errors.New("connect is closed")
//	}
//
//	//read message from chan
//	writeMessage, isOk := <- f.writeChan
//	if isOk && &writeMessage != nil {
//		err := f.conn.WriteMessage(f.msgType, writeMessage.Message)
//		if errors.Is(err, syscall.EPIPE) {
//			//auto connect
//			//f.autoConnect()
//			return err
//		}
//	}
//	return nil
//}
//
//son process for send and receive
//func (f *OneWSClient) runMainProcess() {
//	var (
//		//heartTicker = time.NewTicker(time.Second * define.ClientHeartBeatRate)
//		writeMessage WebSocketMessage
//		isOk                      bool
//		err                       error
//		m any = nil
//	)
//
//	//defer
//	defer func() {
//		if subErr := recover(); subErr != m {
//			log.Printf("WebSocketClient:runMainProcess panic, err:%v", subErr)
//		}
//		//stop and close
//		f.connLocker.Lock()
//		defer f.connLocker.Unlock()
//		//heartTicker.Stop()
//		if f.conn != nil {
//			f.conn.Close()
//			f.conn = nil
//		}
//		f.isConnecting = false
//	}()
//
//	//setup ticker func
//	heatBeatTicker := func() {
//		sf := func() {
//			if f.heartBeatRate > 0 && f.heartBeatChan != nil {
//				f.heartBeatChan <- struct{}{}
//			}
//		}
//		if f.heartBeatRate > 0 {
//			duration := time.Duration(f.heartBeatRate) * time.Second
//			time.AfterFunc(duration, sf)
//		}
//	}
//
//	//main loop
//	//log.Printf("WebSocketClient:runMainProcess start\n")
//	for {
//		select {
//		case writeMessage, isOk = <- f.writeChan:
//			if isOk {
//				//write message
//				err = f.conn.WriteMessage(f.msgType, writeMessage.Message)
//				if err != nil {
//					log.Printf("WebSocketClient:runMainProcess write failed, err:%v", err)
//					if err == io.EOF {
//						return
//					}
//					if errors.Is(err, syscall.EPIPE) {
//						log.Printf("WebSocketClient:runMainProcess write failed, broken pipe error\n")
//						f.autoConnect()
//						return
//					}
//				}
//			}
//		case <- f.closeChan:
//			{
//				return
//			}
//		case <- f.interrupt:
//			{
//				err = f.conn.WriteMessage(
//					websocket.CloseMessage,
//					websocket.FormatCloseMessage(
//						websocket.CloseNormalClosure,
//						"",
//					),
//				)
//				if err != nil {
//					log.Printf("WebSocketClient:runMainProcess write closed, err:%v", err)
//					return
//				}
//			}
//		case <- f.heartBeatChan:
//			{
//				//heart beat
//				f.heartBeat()
//
//				//send next ticker
//				heatBeatTicker()
//			}
//		}
//	}
//}
//
////read message from remote server
//func (f *OneWSClient) readMessageFromServer() {
//	var (
//		messageType int
//		message []byte
//		err error
//		m any = nil
//	)
//
//	//defer
//	defer func() {
//		if subErr := recover(); subErr != m {
//			log.Printf("WebSocketClient:readMessageFromServer panic, err:%v", subErr)
//		}
//	}()
//
//	//loop receive message
//	//log.Printf("WebSocketClient:readMessageFromServer start\n")
//	for {
//		//check
//		if f.conn == nil {
//			//log.Printf("WebSocketClient:readMessageFromServer connect is nil\n")
//			return
//		}
//		if f.forceClosed {
//			return
//		}
//
//		//read original message
//		messageType, message, err = f.conn.ReadMessage()
//		if err != nil {
//			//log.Printf("WebSocketClient:readMessageFromServer failed, connId:%v, err:%v",
//			//	f.connId, err.Error())
//			if errors.Is(err, syscall.EPIPE) {
//				//log.Printf("WebSocketClient:readMessageFromServer write failed, broken pipe error\n")
//				log.Printf("WebSocketClient:readMessageFromServer broken pipe error\n")
//				f.autoConnect()
//				return
//			}
//			if err == io.EOF {
//				log.Printf("WebSocketClient:readMessageFromServer EOF\n")
//				return
//			}
//			if err == websocket.ErrCloseSent {
//				log.Printf("WebSocketClient:readMessageFromServer ErrCloseSent\n")
//				return
//			}
//		}
//
//		//call cb for read message
//		if f.cbForReadMessage != nil {
//			//packet data and send to chan
//			wsMsg := WebSocketMessage{
//				MessageType: messageType,
//				Message: message,
//			}
//			f.cbForReadMessage(&wsMsg)
//		}
//	}
//}