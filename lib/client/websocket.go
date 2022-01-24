package client

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"tubing/define"
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

type WebSocketConnPara struct {
	Host string
	Port int
	Uri  string
	Session string //bound session
	OrgConn *websocket.Conn
}

//client info
type oneWSClient struct {
	//public property
	connPara WebSocketConnPara

	//private property
	u *url.URL
	conn *websocket.Conn
	interrupt chan os.Signal
	readChan chan WebSocketMessage
	writeChan chan WebSocketMessage
	doneChan chan struct{}
	closeChan chan bool
	hasClosed bool
}

//inter websocket manager
type WebSocketClient struct {
	clients sync.Map //session -> oneWSClient, c2s
	orgConn sync.Map //session -> orgConn, s2c
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
	}
	return this
}

//create new c2s client
//client connect the target server
func (f *WebSocketClient) CreateClient(connPara *WebSocketConnPara) (*oneWSClient, error) {
	return newOneWSClient(connPara)
}

//close connect
func (f *WebSocketClient) Close(session string) {
	if session == "" {
		return
	}
	oneWSClient := f.getOneWSClient(session)
	if oneWSClient == nil {
		return
	}
	oneWSClient.close()
}

//////////////////
//private func
//////////////////

//get one websocket client by session
func (f *WebSocketClient) getOneWSClient(session string) *oneWSClient {
	v, ok := f.clients.Load(session)
	if !ok || v == nil {
		return nil
	}
	conn, ok := v.(*oneWSClient)
	if !ok {
		return nil
	}
	return conn
}

//////////////////////
//api for oneWSClient
/////////////////////

func newOneWSClient(connPara *WebSocketConnPara) (*oneWSClient, error) {
	//self init
	this := &oneWSClient{
		connPara: *connPara,
		interrupt: make(chan os.Signal, 1),
		readChan: make(chan WebSocketMessage, define.DefaultChanSize),
		writeChan: make(chan WebSocketMessage, define.DefaultChanSize),
		doneChan: make(chan struct{}),
		closeChan: make(chan bool, 1),
	}
	//dial server
	err := this.dialServer()
	return this, err
}

//send message data
func (f *oneWSClient) SendMessage(messageType int, message[]byte) error {
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
		MessageType: messageType,
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
func (f *oneWSClient) close() {
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
func (f *oneWSClient) dialServer() error {
	//check and init url
	if f.u == nil {
		f.u = &url.URL{
			Scheme: "ws",
			Host: fmt.Sprintf("%s:%d", f.connPara.Host, f.connPara.Port),
			Path: f.connPara.Uri,
		}
	}

	//try dial server
	conn, _, err := websocket.DefaultDialer.Dial(f.u.String(), nil)
	if err != nil {
		return err
	}

	//init interrupt
	signal.Notify(f.interrupt, os.Interrupt)

	//sync object
	f.conn = conn

	//spawn son process
	go f.readMessageFromServer(f.doneChan)
	go f.runMainProcess()
	return nil
}

//cb for read message from target server
func (f *oneWSClient) cbForReadMessage(message *WebSocketMessage) error {
	return nil
}

//son process for send and receive
func (f *oneWSClient) runMainProcess() {
	var (
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
		close(f.writeChan)
		close(f.closeChan)
	}()

	//loop
	for {
		select {
		case readMessage, isOk = <- f.readChan:
			if isOk {
				//read original message
				f.cbForReadMessage(&readMessage)
			}
		case writeMessage, isOk = <- f.writeChan:
			if isOk {
				//write message
				err = f.conn.WriteMessage(writeMessage.MessageType, writeMessage.Message)
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
		}
	}
}

//read message from remote server
func (f *oneWSClient) readMessageFromServer(done chan struct{}) {
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