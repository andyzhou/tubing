package websocket

import (
	"github.com/gorilla/websocket"
	"sync"
)

/*
 * websocket connect
 */

//web socket connect info
type WSConn struct {
	conn *websocket.Conn
	sync.RWMutex
}

//construct
func NewWSConn(conn *websocket.Conn) *WSConn {
	this := &WSConn{
		conn: conn,
	}
	return this
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