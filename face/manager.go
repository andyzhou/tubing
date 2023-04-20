package face

import (
	"errors"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
	"log"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

/*
 * websocket manager
 */

//manager info
type Manager struct {
	msgType int //reference from router
	connId int64
	connMap sync.Map //connId -> IWSConn
	connCount int64
	closeChan chan struct{}
}

//construct
func NewManager() *Manager {
	this := &Manager{
		connMap: sync.Map{},
		closeChan: make(chan struct{}, 1),
	}
	//spawn main process
	go this.runMainProcess()
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

//get cur max conn id
func (f *Manager) GetMaxConnId() int64 {
	return f.connId
}

//gen new conn id
func (f *Manager) GenConnId() int64 {
	return atomic.AddInt64(&f.connId, 1)
}

//get conn count
func (f *Manager) GetConnCount() int64 {
	return f.connCount
}

//set message type
func (f *Manager) SetMessageType(iType int) {
	f.msgType = iType
}

//send message
func (f *Manager) SendMessage(
			message []byte,
			connIds ... int64) error {
	//check
	if message == nil || connIds == nil {
		return errors.New("invalid parameter")
	}
	for _, connId := range connIds {
		conn, err := f.GetConn(connId)
		if err != nil || conn == nil {
			continue
		}
		conn.Write(f.msgType, message)
	}
	return nil
}

//cast message
func (f *Manager) CastMessage(
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
				conn.Write(f.msgType, message)
			}
		}else{
			//all
			conn.Write(f.msgType, message)
		}
		return true
	}
	f.connMap.Range(sf)
	return nil
}

//get conn by id
func (f *Manager) GetConn(connId int64) (IWSConn, error) {
	if connId <= 0 {
		return nil, errors.New("invalid parameter")
	}
	v, ok := f.connMap.Load(connId)
	if !ok || v == nil {
		return nil, errors.New("no such connect")
	}
	conn, ok := v.(IWSConn)
	if !ok {
		return nil, errors.New("invalid ws connect")
	}
	return conn, nil
}

//heart beat
func (f *Manager) HeartBeat(connId int64) error {
	//check
	if connId <= 0 {
		return errors.New("invalid parameter")
	}
	conn, _ := f.GetConn(connId)
	if conn == nil {
		return errors.New("can't get conn by id")
	}
	conn.HeartBeat()
	return nil
}

//accept websocket connect
func (f *Manager) Accept(
				connId int64,
				conn *websocket.Conn) (IWSConn, error) {
	//check
	if connId <= 0 || conn == nil {
		return nil, errors.New("invalid parameter")
	}
	//init new connect
	wsConn := NewWSConn(conn)
	f.connMap.Store(connId, wsConn)
	atomic.AddInt64(&f.connCount, 1)
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
func (f *Manager) CloseConn(connIds ...int64) error {
	//check
	if connIds == nil || len(connIds) <= 0 {
		return errors.New("invalid parameter")
	}
	for _, connId := range connIds {
		//load and update
		v, ok := f.connMap.Load(connId)
		if !ok || v == nil {
			continue
		}
		f.connMap.Delete(connId)
		atomic.AddInt64(&f.connCount, -1)
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
	if f.connCount < 0 {
		atomic.StoreInt64(&f.connCount, 0)
	}
	return nil
}

///////////////
//private func
///////////////

//check un-active conn
func (f *Manager) checkUnActiveConn() {
	sf := func(k, v interface{}) bool {
		conn, ok := v.(IWSConn)
		if ok && conn != nil {
			if !conn.ConnIsActive() {
				//un-active connect
				//close and delete it
				conn.Close()
				f.connMap.Delete(k)
				atomic.AddInt64(&f.connCount, -1)
			}
		}
		return true
	}
	f.connMap.Range(sf)
	if f.connCount < 0 {
		atomic.StoreInt64(&f.connCount, 0)
	}
}

//run main process
func (f *Manager) runMainProcess() {
	var (
		ticker = time.NewTicker(time.Second * define.ServerHeartBeatRate)
	)

	//defer
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Manager:runMainProcess panic err:%v\n", err)
		}
		ticker.Stop()
	}()

	//loop
	for {
		select {
		case <- ticker.C:
			{
				f.checkUnActiveConn()
			}
		case <- f.closeChan:
			return
		}
	}
}