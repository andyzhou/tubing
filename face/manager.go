package face

import (
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket manager
 */

//manager info
type Manager struct {
	msgType int //reference from router
	heartRate int //heart beat rate
	connId int64 //connect id automatic
	connMap sync.Map //connId -> IWSConn
	connTags sync.Map //tag -> map[int64]bool, int64 map
	connCount int64
	heartCheckChan chan struct{}
	heartChan chan int //used for update heart beat
	closeChan chan struct{}
	activeSwitcher bool //used for check conn active or not
}

//construct
func NewManager() *Manager {
	this := &Manager{
		connMap: sync.Map{},
		connTags: sync.Map{},
		heartCheckChan: make(chan struct{}, 1),
		heartChan: make(chan int, 1),
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
	if f.closeChan != nil {
		f.closeChan <- struct{}{}
	}
}

//mark conn tag
func (f *Manager) MarkTag(connId int64, tags ...string) error {
	//check
	if connId <= 0 || tags == nil || len(tags) <= 0 {
		return errors.New("invalid parameter")
	}
	conn, err := f.GetConn(connId)
	if err != nil {
		return err
	}
	if conn == nil {
		return errors.New("no such connect id")
	}

	//mark conn tag
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		v, ok := f.connTags.Load(tag)
		if !ok || v == nil {
			v = map[int64]bool{}
		}
		if subMap, subOk := v.(map[int64]bool); subOk {
			subMap[connId] = true
		}
		f.connTags.Store(tag, v)
	}
	return nil
}

//set active check switch
func (f *Manager) SetActiveSwitch(switcher bool) {
	f.activeSwitcher = switcher
	if switcher {
		//start checker
		f.heartCheckChan <- struct{}{}
	}
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
	var (
		err error
	)
	//check
	if message == nil || connIds == nil || len(connIds) <= 0 {
		return errors.New("invalid parameter")
	}
	for _, connId := range connIds {
		conn, subErr := f.GetConn(connId)
		if subErr != nil || conn == nil {
			continue
		}
		err = conn.Write(f.msgType, message)
	}
	return err
}

//cast message
func (f *Manager) CastMessage(
				message []byte,
				tags ...string) error {
	//check
	if message == nil || len(message) <= 0 {
		return errors.New("invalid parameter")
	}

	//filter by tags first
	if tags != nil && len(tags) > 0 {
		connIds, _ := f.getConnIdsByTag(tags...)
		if connIds != nil {
			for _, connId := range connIds {
				//get conn by id and write message
				conn, _ := f.GetConn(connId)
				if conn != nil {
					conn.Write(f.msgType, message)
				}
			}
		}
		return nil
	}

	//cast message to all
	sf := func(k, v interface{}) bool {
		conn, ok := v.(IWSConn)
		if !ok || conn == nil {
			return true
		}
		//write message
		conn.Write(f.msgType, message)
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

//set heart beat rate
func (f *Manager) SetHeartRate(rate int) error {
	if rate < 0 {
		return errors.New("invalid parameter")
	}
	if f.heartChan == nil {
		return errors.New("heart chan is nil")
	}
	f.heartChan <- rate
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
	log.Printf("manager.CloseConn, connect count:%v\n", f.connCount)
	return nil
}

///////////////
//private func
///////////////

//get conn ids by tag
func (f *Manager) getConnIdsByTag(tags ...string) ([]int64, error) {
	//check
	if tags == nil || len(tags) <= 0 {
		return nil, errors.New("invalid parameter")
	}
	//format result
	result := make([]int64, 0)
	for _, tag := range tags {
		v, ok := f.connTags.Load(tag)
		if ok && v != nil {
			if tempMap, subOk := v.(map[int64]bool); subOk && tempMap != nil {
				for connId, _ := range tempMap {
					result = append(result, connId)
				}
			}
		}
	}
	return result, nil
}

//check un-active conn
func (f *Manager) checkUnActiveConn() {
	sf := func(k, v interface{}) bool {
		conn, ok := v.(IWSConn)
		if ok && conn != nil {
			if f.activeSwitcher && !conn.ConnIsActive(f.heartRate) {
				//un-active connect
				//close and delete it
				log.Printf("tubing.manager:checkUnActiveConn, conn:%v is un-active\n", k)
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
		rate int
		isOk bool
		m any = nil
	)

	//defer
	defer func() {
		if err := recover(); err != m {
			log.Printf("tubing.manager:runMainProcess panic err:%v\n", err)
		}
		if f.heartCheckChan != nil {
			close(f.heartCheckChan)
		}
		if f.heartChan != nil {
			close(f.heartChan)
		}
	}()

	//loop
	for {
		select {
		case <- f.heartCheckChan:
			{
				//check and send next check
				if f.heartRate > 0 && f.activeSwitcher {
					//check un-active connect
					go f.checkUnActiveConn()

					sf := func() {
						f.heartCheckChan <- struct{}{}
					}
					delay := time.Second * time.Duration(f.heartRate)
					time.AfterFunc(delay, sf)
				}
			}
		case rate, isOk = <- f.heartChan:
			if isOk && rate > 0 {
				//sync heart rate
				f.heartRate = rate

				if f.activeSwitcher {
					//resend first heart check
					f.heartCheckChan <- struct{}{}
				}
			}
		case <- f.closeChan:
			return
		}
	}
}