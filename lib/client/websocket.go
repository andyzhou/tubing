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
	"tubing/define"
)

/*
 * websocket client
 * - persistent connect
 */

//message info
type WebSocketMessage struct {
	MessageType int
	Message []byte
}

//client info
type WebSocketClient struct {
	//public property
	Host string
	Port int
	Uri  string
	Session string //bound session

	//private property
	u *url.URL
	conn *websocket.Conn
	cbForRead func(session string, msg *WebSocketMessage)
	interrupt chan os.Signal
	readChan chan WebSocketMessage
	writeChan chan WebSocketMessage
	doneChan chan struct{}
	closeChan chan bool
	hasClosed bool
}

//construct
func NewWebSocketClient() *WebSocketClient {
	this := &WebSocketClient{
		interrupt: make(chan os.Signal, 1),
		readChan: make(chan WebSocketMessage, define.DefaultChanSize),
		writeChan: make(chan WebSocketMessage, define.DefaultChanSize),
		doneChan: make(chan struct{}),
		closeChan: make(chan bool, 1),
	}
	return this
}

//close
func (f *WebSocketClient) Close() {
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

//send message data
func (f *WebSocketClient) SendMessage(messageType int, message[]byte) error {
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

//dial server
func (f *WebSocketClient) DialServer() error {
	//check and init url
	if f.u == nil {
		f.u = &url.URL{
			Scheme: "ws",
			Host: fmt.Sprintf("%s:%d", f.Host, f.Port),
			Path: f.Uri,
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

//set cb for read data
func (f *WebSocketClient) SetCBForRead(cb func(session string, msg *WebSocketMessage)) bool {
	if cb == nil {
		return false
	}
	f.cbForRead = cb
	return true
}

///////////////
//private func
///////////////

//son process for send and receive
func (f *WebSocketClient) runMainProcess() {
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
				if f.cbForRead != nil {
					f.cbForRead(f.Session, &readMessage)
				}
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
func (f *WebSocketClient) readMessageFromServer(done chan struct{}) {
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