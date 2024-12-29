package face

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket manager
 * - one router one manager
 * - manage batch buckets
 * - bucket hashed by ws conn id
 */

//manager info
type Manager struct {
	router      IRouter          //reference from outside
	remote      IRemote          //inter remote and conn contain
	groupMap    map[int64]IGroup //groupId -> IGroup
	bucketMap   map[int]IBucket  //idx -> IBucket
	buckets     int              //running buckets
	msgType     int              //reference from router
	connectId   int64            //atom connect id
	groupLocker sync.RWMutex
	locker      sync.RWMutex
	sync.RWMutex
}

//construct
func NewManager(router IRouter) *Manager {
	this := &Manager{
		router: router,
		remote: NewRemote(),
		groupMap: map[int64]IGroup{},
		bucketMap: map[int]IBucket{},
		msgType: router.GetRouterCfg().MsgType,
	}
	this.interInit()
	return this
}

//quit
func (f *Manager) Quit() {
	//check
	if f.bucketMap == nil {
		return
	}

	//loop clear buckets
	for k, v := range f.bucketMap {
		v.Quit()
		delete(f.bucketMap, k)
	}

	//remote clean up
	f.remote.Cleanup()

	//gc memory
	runtime.GC()
}

func (f *Manager) GetRouter() IRouter {
	return f.router
}

func (f *Manager) GetRemote() IRemote {
	return f.remote
}

//for group
//remove group
func (f *Manager) RemoveGroup(groupId int64) error {
	//check
	if groupId <= 0 {
		return errors.New("invalid parameter")
	}

	//remove with locker
	f.groupLocker.Lock()
	defer f.groupLocker.Unlock()
	delete(f.groupMap, groupId)

	//gc opt
	if len(f.groupMap) <= 0 {
		runtime.GC()
	}

	return nil
}

//get group
func (f *Manager) GetGroup(groupId int64) (IGroup, error) {
	//check
	if groupId <= 0 {
		return nil, errors.New("invalid parameter")
	}

	//get with locker
	f.groupLocker.Lock()
	defer f.groupLocker.Unlock()
	group, ok := f.groupMap[groupId]
	if ok && group != nil {
		return group, nil
	}
	return nil, nil
}

//create new group
func (f *Manager) CreateGroup(groupId int64) error {
	//check
	if groupId <= 0 {
		return errors.New("invalid parameter")
	}

	//get old group
	oldGroup, err := f.GetGroup(groupId)
	if err != nil || oldGroup != nil {
		return errors.New("group has exists")
	}

	//create new group
	f.groupLocker.Lock()
	defer f.groupLocker.Unlock()
	newGroup := NewGroup(groupId)
	f.groupMap[groupId] = newGroup
	return nil
}

//gen new connect id
func (f *Manager) GenConnId() int64 {
	newConnId := atomic.AddInt64(&f.connectId, 1)
	return newConnId
}

////remove conn tag
//func (f *Manager) RemoveTag(connId int64, tags ...string) error {
//	//check
//	if connId <= 0 || tags == nil || len(tags) <= 0 {
//		return errors.New("invalid parameter")
//	}
//	conn, err := f.GetConn(connId)
//	if err != nil {
//		return err
//	}
//	if conn == nil {
//		return errors.New("no such connect id")
//	}
//
//	//remove tags
//	f.mapLock.Lock()
//	defer f.mapLock.Unlock()
//	for _, tag := range tags {
//		tagVal, _ := f.connTagMap.Load(tag)
//		tempMap, _ := tagVal.(map[int64]bool)
//		if tempMap != nil {
//			delete(tempMap, connId)
//		}
//	}
//	conn.RemoveTags(tags...)
//	return nil
//}
//
////mark conn tag
//func (f *Manager) MarkTag(connId int64, tags ...string) error {
//	//check
//	if connId <= 0 || tags == nil || len(tags) <= 0 {
//		return errors.New("invalid parameter")
//	}
//	conn, err := f.GetConn(connId)
//	if err != nil {
//		return err
//	}
//	if conn == nil {
//		return errors.New("no such connect id")
//	}
//
//	//mark conn tag
//	conn.MarkTags(tags...)
//
//	//setup tag
//	for _, tag := range tags {
//		if tag == "" {
//			continue
//		}
//		v, ok := f.connTagMap.Load(tag)
//		if !ok || v == nil {
//			v = map[int64]bool{}
//		}
//		if subMap, subOk := v.(map[int64]bool); subOk {
//			subMap[connId] = true
//		}
//		f.connTagMap.Store(tag, v)
//	}
//	return nil
//}
//
////get cur max conn id
//func (f *Manager) GetMaxConnId() int64 {
//	return f.connId
//}
//
////gen new conn id
//func (f *Manager) GenConnId() int64 {
//	return atomic.AddInt64(&f.connId, 1)
//}
//
////get conn count
//func (f *Manager) GetConnCount() int64 {
//	return f.connCount
//}

//get all buckets
func (f *Manager) GetBuckets() map[int]IBucket {
	return f.bucketMap
}

//get bucket by id
func (f *Manager) GetBucket(bucketId int) (IBucket, error) {
	//check
	if bucketId < 0 {
		return nil, errors.New("invalid parameter")
	}

	//get by id with locker
	f.locker.Lock()
	defer f.locker.Unlock()
	v, ok := f.bucketMap[bucketId]
	if !ok || v == nil {
		return nil, nil
	}
	return v, nil
}

//get conn by id
func (f *Manager) GetConn(
	connId int64) (IWSConn, error) {
	if connId <= 0 {
		return nil, errors.New("invalid parameter")
	}

	//pick target bucket by connect id
	targetBucketId := int(connId % int64(f.buckets))
	targetBucket, err := f.GetBucket(targetBucketId)
	if err != nil || targetBucket == nil {
		return nil, err
	}

	//get target ws connect
	wsConn, subErr := targetBucket.GetConnect(connId)
	return wsConn, subErr
}

//send message
func (f *Manager) SendMessage(
	para *define.SendMsgPara) error {
	var (
		err error
	)
	//check
	if para == nil || para.Msg == nil {
		return errors.New("invalid parameter")
	}

	//send to all buckets
	for _, v := range f.bucketMap {
		err = v.SendMessage(para)
	}
	return err
}

//close conn with message
func (f *Manager) CloseWithMessage(
	conn *websocket.Conn,
	message string) error {
	//check
	if conn == nil {
		return errors.New("invalid parameter")
	}

	//write ws message
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	err := conn.WriteMessage(websocket.CloseMessage, msg)
	if err != nil {
		return err
	}

	//get conn id by remote addr
	remoteAddr := conn.RemoteAddr().String()
	connId, _ := f.getConnIdByRemoteAddr(remoteAddr)
	if connId <= 0 {
		return errors.New("can't get connect id by remote address")
	}

	//pick target bucket by connect id
	targetBucketId := int(connId % int64(f.buckets))
	targetBucket, subErr := f.GetBucket(targetBucketId)
	if subErr != nil || targetBucket == nil {
		return subErr
	}

	//close connect
	targetBucket.CloseConnect(connId)

	//delete remote addr
	err = f.remote.DelRemote(remoteAddr)
	return err
}

//close conn by ids
func (f *Manager) CloseConnect(connIds ...int64) error {
	var (
		targetBucket IBucket
		err error
	)
	//check
	if connIds == nil || len(connIds) <= 0 {
		return errors.New("invalid parameter")
	}

	//close one by one
	for _, connId := range connIds {
		//get target bucket by connect id
		targetBucketId := int(connId % int64(f.buckets))
		if targetBucketId < 0 {
			continue
		}
		targetBucket, err = f.GetBucket(targetBucketId)
		if err != nil || targetBucket == nil {
			continue
		}
		remoteAddrMap, _ := targetBucket.CloseConnect(connId)
		if remoteAddrMap != nil {
			for _, addr := range remoteAddrMap {
				f.remote.DelRemote(addr)
			}
		}
	}
	return nil
}

//accept new websocket connect
func (f *Manager) Accept(connId int64, conn *websocket.Conn, ctx *gin.Context) (IWSConn, error) {
	//check
	if connId <= 0 || conn == nil {
		return nil, fmt.Errorf("invalid parameter, connid:%v, conn:%v\n", connId, conn)
	}
	connRemoteAddr := conn.RemoteAddr().String()

	//init new connect
	wsConn := NewWSConn(conn, connId, ctx)
	err := wsConn.SetRemoteAddr(connRemoteAddr)
	if err != nil {
		return nil, err
	}

	//pick target bucket by connect id
	targetBucketId := int(connId % int64(f.buckets))
	targetBucket, subErr := f.GetBucket(targetBucketId)
	if subErr != nil || targetBucket == nil {
		return nil, subErr
	}

	//add remote addr
	err = f.remote.AddRemote(connRemoteAddr, connId)

	//add new ws connect on target bucket
	err = targetBucket.AddConnect(wsConn)
	return wsConn, err
}

//set cb for read messages
func (f *Manager) SetCBForReadMessage(cb func(string, int64, IWSConn, int, []byte, *gin.Context) error) {
	for _, v := range f.bucketMap {
		v.SetCBForReadMessage(cb)
	}
}

//set cb for connect closed
func (f *Manager) SetCBForConnClosed(cb func(string, int64, IWSConn, *gin.Context) error) {
	for _, v := range f.bucketMap {
		v.SetCBForConnClosed(cb)
	}
}

//set message type
func (f *Manager) SetMessageType(iType int) {
	f.msgType = iType
	for _, v := range f.bucketMap {
		v.SetMsgType(iType)
	}
}

////close conn
//func (f *Manager) CloseConn(connIds ...int64) error {
//	//check
//	if connIds == nil || len(connIds) <= 0 {
//		return errors.New("invalid parameter")
//	}
//
//	//loop check and close with locker
//	f.mapLock.Lock()
//	defer f.mapLock.Unlock()
//	needGc := false
//	for _, connId := range connIds {
//		//load and update
//		conn, ok := f.connMap[connId]
//		if !ok || conn == nil {
//			continue
//		}
//
//		//close ws connect
//		conn.Close()
//		remoteAddr := conn.GetRemoteAddr()
//		if remoteAddr != "" {
//			delete(f.connRemoteMap, remoteAddr)
//		}
//
//		//remove from bucket
//		f.router.GetBucket().RemoveConnect(connId)
//
//		//remove relate data
//		delete(f.connMap, connId)
//		atomic.AddInt64(&f.connCount, -1)
//		needGc = true
//	}
//
//	//check and release conn map
//	if f.connCount <= 0 {
//		f.connMap = map[int64]IWSConn{}
//		f.connRemoteMap = map[string]int64{}
//		atomic.StoreInt64(&f.connCount, 0)
//		if needGc {
//			log.Printf("tubing.server.manager.CloseConn, gc opt\n")
//			runtime.GC()
//		}
//	}
//	return nil
//}
//
/////////////////
////private func
/////////////////
//
////get conn ids by tag
//func (f *Manager) getConnIdsByTag(tags ...string) ([]int64, error) {
//	//check
//	if tags == nil || len(tags) <= 0 {
//		return nil, errors.New("invalid parameter")
//	}
//	//format result
//	result := make([]int64, 0)
//	for _, tag := range tags {
//		v, ok := f.connTagMap.Load(tag)
//		if ok && v != nil {
//			if tempMap, subOk := v.(map[int64]bool); subOk && tempMap != nil {
//				for connId, _ := range tempMap {
//					result = append(result, connId)
//				}
//			}
//		}
//	}
//	return result, nil
//}
//
////send active message
//func (f *Manager) sendActiveMsg(conn IWSConn) {
//	messageType := define.MessageTypeOfJson
//	activeMsg := fmt.Sprintf("active %v", time.Now().String())
//	message := []byte(activeMsg)
//	err := conn.Write(messageType, message)
//	if err != nil {
//		log.Printf("manager.sendActiveMsg failed, err:%v\n", err.Error())
//	}
//}
//
////cb for active check ticker
//func (f *Manager) cbForActiveCheck() error {
//	log.Printf("manager.cbForActiveCheck..\n")
//	if f.connCount <= 0 {
//		return nil
//	}
//	f.mapLock.Lock()
//	defer f.mapLock.Unlock()
//	for _, conn := range f.connMap {
//		log.Printf("manager.cbForActiveCheck, conn id:%v\n", conn.GetConnId())
//		if !conn.ConnIsActive(f.heartRate) {
//			//force close un-active connect
//			log.Printf("manager.cbForActiveCheck, force close un-active conn\n")
//			//f.CloseConn(conn.GetConnId())
//		}else{
//			//active connect
//			//f.sendActiveMsg(conn)
//		}
//	}
//	return nil
//}

//get connect id by remote addr
func (f *Manager) getConnIdByRemoteAddr(remoteAddr string) (int64, error) {
	//check
	if remoteAddr == "" {
		return 0, errors.New("invalid parameter")
	}
	v, err := f.remote.GetRemote(remoteAddr)
	return v, err
}

//inter init
func (f *Manager) interInit() {
	//create batch buckets
	buckets := f.router.GetConf().Buckets
	if buckets <= 0 {
		buckets = define.DefaultBuckets
	}
	f.buckets = buckets

	//init all buckets
	for i := 0; i < buckets; i++ {
		bucket := NewBucket(i, f)
		f.bucketMap[i] = bucket
	}
}