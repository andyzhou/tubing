package face

import (
	"errors"
	"github.com/gorilla/websocket"
	"reflect"
	"sync"
)

/*
 * websocket manager
 */

//manager info
type Manager struct {
	connMap sync.Map //session -> IWSConn
}

//construct
func NewManager() *Manager {
	this := &Manager{
		connMap: sync.Map{},
	}
	return this
}

//close
func (f *Manager) Close() {
	sf := func(k, v interface{}) bool {
		conn, _ := v.(IWSConn)
		if conn != nil {
			conn.Close()
		}
		return true
	}
	f.connMap.Range(sf)
	f.connMap = sync.Map{}
}

//send message
func (f *Manager) SendMessage(
			messageType int,
			message []byte,
			sessions ... string) error {
	//check
	if message == nil || sessions == nil {
		return errors.New("invalid parameter")
	}
	for _, session := range sessions {
		conn, err := f.GetConn(session)
		if err != nil || conn == nil {
			continue
		}
		conn.Write(messageType, message)
	}
	return nil
}

//cast message
func (f *Manager) CastMessage(
				messageType int,
				message []byte,
				tags ...string) error {
	//check
	if message == nil {
		return errors.New("invalid parameter")
	}
	sf := func(k, v interface{}) bool {
		conn, ok := v.(IWSConn)
		if !ok || conn == nil {
			return true
		}
		if tags != nil {
			//filter by tags
			bRet := reflect.DeepEqual(tags, conn.GetTags())
			if bRet {
				//match relate tags
				conn.Write(messageType, message)
			}
		}else{
			//all
			conn.Write(messageType, message)
		}
		return true
	}
	f.connMap.Range(sf)
	return nil
}

//get conn by session
func (f *Manager) GetConn(session string) (IWSConn, error) {
	if session == "" {
		return nil, errors.New("invalid parameter")
	}
	v, ok := f.connMap.Load(session)
	if !ok || v == nil {
		return nil, errors.New("no such connect")
	}
	conn, ok := v.(IWSConn)
	if !ok {
		return nil, errors.New("invalid ws connect")
	}
	return conn, nil
}

//accept websocket connect
func (f *Manager) Accept(
				session string,
				conn *websocket.Conn) (IWSConn, error) {
	//check
	if session == "" || conn == nil {
		return nil, errors.New("invalid parameter")
	}
	//check session, todo..

	//init new connect
	wsConn := NewWSConn(conn)
	f.connMap.Store(session, wsConn)

	return wsConn, nil
}

//close conn with message
func (f *Manager) CloseWithMessage(
			conn *websocket.Conn,
			message string) error {
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	err := conn.WriteMessage(websocket.CloseMessage, msg)
	if err != nil {
		return err
	}
	return conn.Close()
}

//close conn
func (f *Manager) CloseConn(sessions ...string) error {
	//check
	if sessions == nil {
		return errors.New("invalid parameter")
	}
	for _, session := range sessions {
		//load and update
		v, ok := f.connMap.Load(session)
		if !ok || v == nil {
			continue
		}
		f.connMap.Delete(session)
		wsConn, ok := v.(*WSConn)
		if !ok || wsConn == nil {
			continue
		}
		//begin close and clear
		err := wsConn.Close()
		if err != nil {
			continue
		}
	}
	return nil
}

///////////////
//private func
///////////////