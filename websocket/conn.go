package websocket

import (
	"errors"
	"github.com/gorilla/websocket"
	"sync"
)

/*
 * websocket connect
 */

//web socket connect info
type WSConn struct {
	conn *websocket.Conn
	tags []string
	sync.RWMutex
}

//construct
func NewWSConn(conn *websocket.Conn) *WSConn {
	this := &WSConn{
		conn: conn,
		tags: []string{},
	}
	return this
}

//mark tag
func (f *WSConn) MarkTag(tags ...string) error {
	if tags == nil || len(tags) <= 0 {
		return errors.New("invalid parameter")
	}
	f.tags = []string{}
	f.tags = append(f.tags, tags...)
	return nil
}

//write data
func (f *WSConn) Write(messageType int, data []byte) error {
	f.Lock()
	defer f.Unlock()
	return f.conn.WriteMessage(messageType, data)
}

//read data
//return messageType, data, error
func (f *WSConn) Read() (int, []byte, error) {
	return f.conn.ReadMessage()
}

//close conn
func (f *WSConn) CloseWithMessage(message string) error {
	f.Lock()
	defer f.Unlock()
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	err := f.conn.WriteMessage(websocket.CloseMessage, msg)
	if err != nil {
		return err
	}
	return f.Close()
}
func (f *WSConn) Close() error {
	return f.conn.Close()
}