package face

import (
	"bytes"
	"errors"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
	"log"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/andyzhou/tinylib/queue"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * connect bucket
 * - one router batch buckets
 * - one bucket contain batch ws connect
 * - use list and ticker mode for send and read msg
 */

//face info
type Bucket struct {
	//inter obj
	bucketId int
	msgType int //reference from router
	router IRouter //reference from outside
	activeCheckTicker *queue.Ticker //ticker for check active connect
	readMsgTicker *queue.Ticker //ticker for read connect msg
	sendMsgQueue *queue.List //inter queue list for send message

	//run env data
	connMap map[int64]IWSConn //connId -> IWSConn
	connRemoteMap map[string]int64 //remoteAddr -> connId
	connCount int64

	//cb func
	cbForReadMessage func(string, int64, int, []byte) error
	cbForConnClosed func(string, int64) error
	sync.RWMutex
}

//construct
func NewBucket(id int, router IRouter) *Bucket {
	this := &Bucket{
		bucketId: id,
		router: router,
		connMap: map[int64]IWSConn{},
		connRemoteMap: map[string]int64{},
	}
	this.interInit()
	return this
}

//quit
func (f *Bucket) Quit() {
	//close inter ticker
	if f.readMsgTicker != nil {
		f.readMsgTicker.Quit()
	}
	if f.activeCheckTicker != nil {
		f.activeCheckTicker.Quit()
	}

	//free memory
	f.freeRunMemory()
}

//////////////////
//api for cb func
//////////////////

//set cb for read message, step-1-1
//cb func(routeName, connectId, msgType, msgData) error
func (f *Bucket) SetCBForReadMessage(cb func(string, int64, int, []byte) error)  {
	if cb == nil {
		return
	}
	f.cbForReadMessage = cb
}

//set cb for conn closed, step-1-2
func (f *Bucket) SetCBForConnClosed(cb func(string, int64) error) {
	if cb == nil {
		return
	}
	f.cbForConnClosed = cb
}

//set message type
func (f *Bucket) SetMsgType(msgType int) {
	f.msgType = msgType
}

///////////////////////
//api for send message
///////////////////////

//send sync message
func (f *Bucket) SendMessage(para *define.SendMsgPara) error {
	//check
	if para == nil || para.Msg == nil {
		return errors.New("invalid parameter")
	}
	if f.sendMsgQueue == nil {
		return errors.New("inter send message queue is nil")
	}

	//save into running queue
	err := f.sendMsgQueue.Push(para)
	return err
}

//////////////////
//api for connect
//////////////////

//get all connections
func (f *Bucket) GetAllConnect() map[int64]IWSConn {
	return f.connMap
}

//get connect by id
func (f *Bucket) GetConnect(connId int64) (IWSConn, error) {
	//check
	if connId <= 0 {
		return nil, errors.New("invalid parameter")
	}

	//get ws conn by id
	f.Lock()
	defer f.Unlock()
	v, _ := f.connMap[connId]
	return v, nil
}

//close conn with message
func (f *Bucket) CloseWithMessage(conn *websocket.Conn, message string) error {
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

	//get relate data by remote addr
	remoteAddr := conn.RemoteAddr().String()

	//get connect id by remote address
	f.Lock()
	defer f.Unlock()
	connId, _ := f.connRemoteMap[remoteAddr]
	if connId > 0 {
		//close connect from manager
		f.CloseConnect(connId)
	}
	return nil
}

//close connect
//return map[connId]remoteAddr, error
func (f *Bucket) CloseConnect(connIds ...int64) (map[int64]string, error) {
	//check
	if connIds == nil || len(connIds) <= 0 {
		return nil, errors.New("invalid parameter")
	}

	//loop opt
	succeed := 0
	result := make(map[int64]string)
	for _, connId := range connIds {
		//load and update
		conn, ok := f.connMap[connId]
		if !ok || conn == nil {
			continue
		}

		//connect close
		conn.Close()

		//run env data clean
		remoteAddr := conn.GetRemoteAddr()
		if remoteAddr != "" {
			delete(f.connRemoteMap, remoteAddr)
			result[connId] = remoteAddr
		}
		delete(f.connMap, connId)
		atomic.AddInt64(&f.connCount, -1)
		succeed++
	}

	//check and run gc
	if succeed > 0 && f.connCount <= 0 {
		f.connMap = map[int64]IWSConn{}
		f.connRemoteMap = map[string]int64{}
		atomic.StoreInt64(&f.connCount, 0)
		runtime.GC()
		log.Printf("tubing.server.manager.CloseConn, gc opt\n")
	}
	return result, nil
}

//add new connect
func (f *Bucket) AddConnect(conn IWSConn) error {
	//check
	if conn == nil || conn.GetConnId() <= 0 {
		return errors.New("invalid parameter")
	}

	//get key data
	connectId := conn.GetConnId()
	remoteAddr := conn.GetRemoteAddr()

	//add into running data
	f.Lock()
	defer f.Unlock()
	f.connMap[connectId] = conn
	f.connRemoteMap[remoteAddr] = connectId
	atomic.AddInt64(&f.connCount, 1)
	return nil
}

///////////////
//private func
///////////////

//close connect
func (f *Bucket) closeConnect(conn IWSConn) error {
	//check
	if conn == nil {
		return errors.New("invalid parameter")
	}

	//close ws connect
	defer conn.Close()

	//remove from run env
	f.Lock()
	defer f.Unlock()
	remoteAddr := conn.GetRemoteAddr()
	if remoteAddr != "" {
		delete(f.connRemoteMap, remoteAddr)
	}

	delete(f.connMap, conn.GetConnId())
	atomic.AddInt64(&f.connCount, -1)
	return nil
}

//cb for read connect data
//this will reduce tcp read resource cost
func (f *Bucket) cbForReadConnData() error {
	var (
		messageType int
		message []byte
		err error
	)
	//check
	if f.connCount <= 0 || f.connMap == nil {
		return errors.New("no any active connections")
	}

	//loop read connect data
	hasCloseOpt := false
	for connId, conn := range f.connMap {
		//check connect
		if connId <= 0 || conn == nil {
			continue
		}
		connObj, subOk := conn.(*WSConn)
		if !subOk || connObj == nil {
			continue
		}

		//read message
		messageType, message, err = connObj.Read()
		if err != nil {
			//close connect and remove it
			connObj.Close()

			//check and call closed cb
			if f.cbForConnClosed != nil {
				f.cbForConnClosed(f.router.GetName(), connId)
			}

			//remove from bucket
			f.closeConnect(conn)
			hasCloseOpt = true
			continue
		}
		if bytes.Compare(f.router.GetHeartByte(), message) == 0 {
			//it's heart beat data
			connObj.HeartBeat()
			continue
		}

		//check and call read message cb
		if f.cbForReadMessage != nil {
			f.cbForReadMessage(f.router.GetName(), connId, messageType, message)
		}
	}

	//check gc or not
	if hasCloseOpt && f.connCount <= 0 {
		f.freeRunMemory()
	}
	return err
}

//cb for send msg consumer
func (f *Bucket) cbForConsumerSendData(data interface{}) error {
	var (
		checkPass bool
		err error
	)
	//check
	if data == nil {
		return errors.New("invalid parameter")
	}
	sendPara, ok := data.(*define.SendMsgPara)
	if !ok || sendPara == nil || sendPara.Msg == nil {
		return errors.New("invalid parameter")
	}

	//loop send
	for _, v := range f.connMap {
		//check
		if v == nil {
			continue
		}
		//check send condition
		checkPass = f.checkSendCondition(sendPara, v)
		if !checkPass {
			continue
		}
		//send message to target connect
		err = v.Write(f.msgType, sendPara.Msg)
		if err != nil {
			log.Printf("bucket.cbForConsumerSendData failed, err:%v\n", err.Error())
		}
	}
	return err
}

//check send condition
//if check pass, return true or false
func (f *Bucket) checkSendCondition(para *define.SendMsgPara, conn IWSConn) bool {
	//check
	if para == nil || conn == nil {
		return false
	}
	if para.ReceiverIds == nil &&
		para.Tags == nil &&
		para.Property == nil {
		//not need condition check
		return true
	}

	//check by receiver ids
	if len(para.ReceiverIds) > 0 {
		connOwnerId := conn.GetOwnerId()
		if connOwnerId <= 0 {
			return false
		}
		for _, receiverId := range para.ReceiverIds {
			if receiverId == connOwnerId {
				return true
			}
		}
		return false
	}

	//check by tags
	if len(para.Tags) > 0 {
		connTags := conn.GetTags()
		if connTags == nil || len(connTags) <= 0 {
			return false
		}
		for _, tag := range para.Tags {
			if v, ok := connTags[tag]; ok && v {
				return true
			}
		}
		return false
	}

	//check by property
	if len(para.Property) > 0 {
		connProp := conn.GetAllProp()
		if connProp == nil || len(connProp) <= 0 {
			return false
		}
		for k, v := range para.Property {
			sv, ok := connProp[k]
			if ok && sv == v {
				return true
			}
		}
		return false
	}
	return true
}

//free run memory
func (f *Bucket) freeRunMemory() {
	//free memory
	f.Lock()
	defer f.Unlock()
	f.connMap = map[int64]IWSConn{}
	f.connRemoteMap = map[string]int64{}
	runtime.GC()
}

//cb for active connect check
func (f *Bucket) cbForCheckActiveConn() error {
	succeed := 0
	for _, conn := range f.connMap {
		isActive := conn.ConnIsActive()
		if !isActive {
			//force close connect
			f.closeConnect(conn)
			succeed++
		}
	}
	//force gc
	if succeed > 0 {
		runtime.GC()
	}
	return nil
}

//init read message ticker
func (f *Bucket) initReadMsgTicker() {
	//get connect read msg rate
	readMsgRate := f.router.GetConf().ReadByteRate
	if readMsgRate <= 0 {
		readMsgRate = define.DefaultReadMsgTicker
	}

	//init read msg ticker
	f.readMsgTicker = queue.NewTicker(readMsgRate)
	f.readMsgTicker.SetCheckerCallback(f.cbForReadConnData)
}

//init send message queue and consumer
func (f *Bucket) initSendMsgConsumer() {
	//get send msg rate
	sendMsgRate := f.router.GetConf().SendByteRate
	if sendMsgRate <= 0 {
		sendMsgRate = define.DefaultSendMsgTicker
	}
	f.sendMsgQueue = queue.NewList()
	f.sendMsgQueue.SetConsumer(f.cbForConsumerSendData, sendMsgRate)
}

//init active conn check ticker
func (f *Bucket) initActiveConnCheckTicker() {
	//check active rate from config
	activeCheckRate := f.router.GetConf().CheckActiveRate
	if activeCheckRate <= 0 {
		//not need check, do nothing
		return
	}

	//init ticker
	f.activeCheckTicker = queue.NewTicker(float64(activeCheckRate))
	f.activeCheckTicker.SetCheckerCallback(f.cbForCheckActiveConn)
}

//inter init
func (f *Bucket) interInit() {
	//init read msg ticker
	f.initReadMsgTicker()

	//init send message queue
	f.initSendMsgConsumer()

	//init active conn check ticker
	f.initActiveConnCheckTicker()
}