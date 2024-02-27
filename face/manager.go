package face

import (
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket manager
 * bucket hashed by ws conn id
 */

//manager info
type Manager struct {
	router IRouter //reference from outside
	msgType int //reference from router
	heartRate int //heart beat rate
	connId int64 //connect id automatic
	connMap map[int64]IWSConn //connId -> IWSConn
	connRemoteMap map[string]int64 //remoteAddr -> connId
	connTagMap sync.Map //tag -> map[int64]bool => connId -> bool
	connCount int64
	heartCheckChan chan struct{}
	heartChan chan int //used for update heart beat rate
	closeChan chan struct{}
	activeSwitcher bool //used for check conn active or not
	mapLock sync.RWMutex
}

//construct
func NewManager(router IRouter) *Manager {
	this := &Manager{
		router: router,
		connMap: map[int64]IWSConn{},
		connRemoteMap: map[string]int64{},
		connTagMap: sync.Map{},
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
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	for k, v := range f.connMap {
		v.Close()
		delete(f.connMap, k)
	}
	for k, _ := range f.connRemoteMap {
		delete(f.connRemoteMap, k)
	}
	f.connMap = map[int64]IWSConn{}
	f.connRemoteMap = map[string]int64{}
	if f.closeChan != nil {
		f.closeChan <- struct{}{}
	}
}

//remove conn tag
func (f *Manager) RemoveTag(connId int64, tags ...string) error {
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

	//remove tags
	for _, tag := range tags {
		tagVal, _ := f.connTagMap.Load(tag)
		tempMap, _ := tagVal.(map[int64]bool)
		if tempMap != nil {
			f.mapLock.Lock()
			delete(tempMap, connId)
			f.mapLock.Unlock()
		}
	}
	conn.RemoveTags(tags...)
	return nil
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
	conn.MarkTags(tags...)

	//setup tag
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		v, ok := f.connTagMap.Load(tag)
		if !ok || v == nil {
			v = map[int64]bool{}
		}
		if subMap, subOk := v.(map[int64]bool); subOk {
			subMap[connId] = true
		}
		f.connTagMap.Store(tag, v)
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
	var (
		err error
	)
	//check
	if message == nil || len(message) <= 0 {
		return errors.New("invalid parameter")
	}

	//filter by tags first
	if tags != nil && len(tags) > 0 {
		//get matched connect ids
		connIds, _ := f.getConnIdsByTag(tags...)
		if connIds == nil || len(connIds) <= 0 {
			return errors.New("no any matched conn ids by tag")
		}
		//cast to matched connect ids
		for _, connId := range connIds {
			//get conn by id and write message
			conn, _ := f.GetConn(connId)
			if conn != nil {
				err = conn.Write(f.msgType, message)
			}
		}
		return err
	}

	//cast message to all
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	for _, v := range f.connMap {
		if v == nil {
			continue
		}
		//write message
		err = v.Write(f.msgType, message)
	}
	return err
}

//get conn by id
func (f *Manager) GetConn(connId int64) (IWSConn, error) {
	if connId <= 0 {
		return nil, errors.New("invalid parameter")
	}
	//get with locker
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	conn, ok := f.connMap[connId]
	if !ok || conn == nil {
		return nil, errors.New("no such connect")
	}
	return conn, nil
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
	connRemoteAddr := conn.RemoteAddr().String()

	//init new connect
	wsConn := NewWSConn(conn, connId)
	err := wsConn.SetRemoteAddr(connRemoteAddr)

	//add map with locker
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	f.connMap[connId] = wsConn
	f.connRemoteMap[connRemoteAddr] = connId
	atomic.AddInt64(&f.connCount, 1)

	//add new connect into bucket
	f.router.GetBucket().AddConnect(wsConn)
	return wsConn, err
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
	//get relate data by remote addr
	remoteAddr := conn.RemoteAddr().String()

	//get connect id by remote address
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	connId, _ := f.connRemoteMap[remoteAddr]
	if connId > 0 {
		f.CloseConn(connId)

		//remove from bucket
		f.router.GetBucket().RemoveConnect(connId)
	}
	return conn.Close()
}

//close conn
func (f *Manager) CloseConn(connIds ...int64) error {
	//check
	if connIds == nil || len(connIds) <= 0 {
		return errors.New("invalid parameter")
	}
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	needGc := false
	for _, connId := range connIds {
		//load and update
		conn, ok := f.connMap[connId]
		if !ok || conn == nil {
			continue
		}

		//close ws connect
		conn.Close()
		remoteAddr := conn.GetRemoteAddr()
		if remoteAddr != "" {
			delete(f.connRemoteMap, remoteAddr)
		}

		//remove from bucket
		f.router.GetBucket().RemoveConnect(connId)

		//remove relate data
		delete(f.connMap, connId)
		needGc = true
	}
	if f.connCount <= 0 {
		f.connMap = map[int64]IWSConn{}
		f.connRemoteMap = map[string]int64{}
		atomic.StoreInt64(&f.connCount, 0)
		if needGc {
			runtime.GC()
		}
	}
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
		v, ok := f.connTagMap.Load(tag)
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
	f.mapLock.Lock()
	defer f.mapLock.Unlock()
	needGc := false
	for k, conn := range f.connMap {
		if f.activeSwitcher && !conn.ConnIsActive(f.heartRate) {
			//un-active connect
			//close and delete it
			log.Printf("tubing.manager:checkUnActiveConn, conn:%v is un-active\n", k)
			f.CloseConn(conn.GetConnId())
			delete(f.connMap, k)
			atomic.AddInt64(&f.connCount, -1)
			needGc = true
		}
	}
	if f.connCount <= 0 {
		atomic.StoreInt64(&f.connCount, 0)
		if needGc {
			runtime.GC()
		}
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

					//send next heart beta check ticker
					sf := func() {
						if f.heartCheckChan != nil {
							f.heartCheckChan <- struct{}{}
						}
					}
					delay := time.Second * time.Duration(f.heartRate)
					time.AfterFunc(delay, sf)
				}
			}
		case rate, isOk = <- f.heartChan:
			if isOk && rate >= 0 {
				//sync heart rate
				f.heartRate = rate
				if rate == 0 {
					//stop heart check
					f.activeSwitcher = false
				}
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