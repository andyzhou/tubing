package face

import (
	"errors"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * one websocket connect
 * reflect.DeepEqual(x, y)
 */

//web socket connect info
type WSConn struct {
	bucket     IBucket         //parent bucket reference
	connId     int64           //connect id
	ownerId    int64           //conn owner id
	groupId    int64           //temp group id
	ctx        *gin.Context    //reference
	conn       *websocket.Conn //reference
	propMap    map[string]interface{}
	tagMap     map[string]bool
	remoteAddr string
	activeTime int64
	tagLock    sync.RWMutex
	propLock   sync.RWMutex
	//sync.RWMutex
}

//construct
func NewWSConn(bucket IBucket, connId int64, conn *websocket.Conn, ctx *gin.Context) *WSConn {
	this := &WSConn{
		bucket: bucket,
		connId: connId,
		conn: conn,
		ctx: ctx,
		propMap: map[string]interface{}{},
		tagMap: map[string]bool{},
	}
	go this.readDataProcess()
	return this
}

//close conn
func (f *WSConn) CloseWithMessage(message string) error {
	//close with locker
	//f.Lock()
	//defer f.Unlock()
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	f.conn.WriteMessage(websocket.CloseMessage, msg)
	return f.Close()
}
func (f *WSConn) Close() error {
	//close with locker
	//f.Lock()
	//defer f.Unlock()
	err := f.conn.Close()

	//release memory
	f.propLock.Lock()
	f.propMap = map[string]interface{}{}
	f.propLock.Unlock()

	f.tagLock.Lock()
	f.tagMap = map[string]bool{}
	f.tagLock.Unlock()

	//gc opt
	runtime.GC()
	return err
}

//set get group id
func (f *WSConn) GetGroupId() int64 {
	return f.groupId
}
func (f *WSConn) SetGroupId(groupId int64) {
	f.groupId = groupId
}

//get connect id
func (f *WSConn) GetConnId() int64 {
	return f.connId
}

//get connect
func (f *WSConn) GetConnect() *websocket.Conn {
	//f.Lock()
	//defer f.Unlock()
	return f.conn
}

//get gin context
func (f *WSConn) GetContext() *gin.Context {
	return f.ctx
}

//get remote addr
func (f *WSConn) GetRemoteAddr() string {
	return f.remoteAddr
}

//set remote addr
func (f *WSConn) SetRemoteAddr(addr string) error {
	if addr == "" {
		return errors.New("invalid parameter")
	}
	f.remoteAddr = addr
	return nil
}

//get tags
func (f *WSConn) GetTags() map[string]bool {
	f.tagLock.Lock()
	defer f.tagLock.Unlock()
	return f.tagMap
}

//remove tags
func (f *WSConn) RemoveTags(tags ...string) error {
	//check
	if tags == nil || len(tags) <= 0 {
		return errors.New("invalid parameter")
	}

	//del mark with locker
	f.tagLock.Lock()
	defer f.tagLock.Unlock()
	for _, tag := range tags {
		delete(f.tagMap, tag)
	}

	//gc opt
	if len(f.tagMap) <= 0 {
		runtime.GC()
	}
	return nil
}

//mark tags
func (f *WSConn) MarkTags(tags ...string) error {
	//check
	if tags == nil || len(tags) <= 0 {
		return errors.New("invalid parameter")
	}

	//mark with locker
	f.tagLock.Lock()
	defer f.tagLock.Unlock()
	for _, tag := range tags {
		f.tagMap[tag] = true
	}
	return nil
}

//get owner id
func (f *WSConn) GetOwnerId() int64 {
	return f.ownerId
}

//set owner id
func (f *WSConn) SetOwnerId(ownerId int64) error {
	//check
	if ownerId <= 0 {
		return errors.New("invalid parameter")
	}
	f.ownerId = ownerId
	return nil
}

//verify properties
//if exists return true, or face
func (f *WSConn) VerifyProp(keys ...string) bool {
	//check
	if keys == nil || len(keys) <= 0 {
		return false
	}

	//loop verify with locker
	f.propLock.Lock()
	defer f.propLock.Unlock()
	for _, key := range keys {
		_, ok := f.propMap[key]
		if ok {
			return true
		}
	}
	return false
}

//get all property
func (f *WSConn) GetAllProp() map[string]interface{} {
	//get with locker
	f.propLock.Lock()
	defer f.propLock.Unlock()
	return f.propMap
}

//get property
func (f *WSConn) GetProp(key string) (interface{}, error) {
	//check
	if key == "" {
		return nil, errors.New("invalid parameter")
	}

	//get prop with locker
	f.propLock.Lock()
	defer f.propLock.Unlock()
	v, ok := f.propMap[key]
	if ok && v != nil{
		return v, nil
	}
	return nil, errors.New("no such property")
}

//del property
func (f *WSConn) DelProp(key string) error {
	//check
	if key == "" {
		return errors.New("invalid parameter")
	}

	//del prop with locker
	f.propLock.Lock()
	defer f.propLock.Unlock()
	delete(f.propMap, key)

	//gc opt
	if len(f.propMap) <= 0 {
		runtime.GC()
	}
	return nil
}

//set property
func (f *WSConn) SetProp(key string, val interface{}) error {
	//check
	if key == "" || val == nil || val == "" {
		return errors.New("invalid parameter")
	}

	//set prop with locker
	f.propLock.Lock()
	defer f.propLock.Unlock()
	f.propMap[key] = val
	return nil
}

//check conn is active
func (f *WSConn) ConnIsActive(checkRates ...int) bool {
	var (
		checkRate int
	)
	if checkRates != nil && len(checkRates) > 0 {
		checkRate = checkRates[0]
	}
	if checkRate <= 0 {
		checkRate = define.DefaultHeartBeatRate
	}
	now := time.Now().Unix()
	diff := now - f.activeTime
	if diff >= int64(checkRate) {
		return false
	}
	return true
}

//heart beat for update active time
func (f *WSConn) HeartBeat() {
	atomic.StoreInt64(&f.activeTime, time.Now().Unix())
}

//write data
func (f *WSConn) Write(messageType int, data []byte) error {
	//check
	if f.conn == nil {
		return errors.New("invalid ws connect")
	}

	//write message with locker
	//f.Lock()
	//defer f.Unlock()
	defer func() {
		atomic.StoreInt64(&f.activeTime, time.Now().Unix())
	}()

	//f.conn.SetWriteDeadline(time.Now().Add(time.Duration(float64(time.Second))))
	err := f.conn.WriteMessage(messageType, data)

	//debug and trace
	//dataStr := string(data)
	//log.Printf("ws.conn, id:%v, write:%v\n", f.connId, dataStr)
	return err
}

//read data
//return messageType, data, error
func (f *WSConn) Read() (int, []byte, error) {
	//check
	if f.conn == nil {
		return 0, nil, errors.New("invalid ws connect")
	}

	//read message with locker
	//f.Lock()
	//defer f.Unlock()
	//f.conn.SetReadDeadline(time.Now().Add(time.Duration(float64(time.Second))))
	return f.conn.ReadMessage()
}

//read one message data from client side
func (f *WSConn) readOneMessageData() error {
	//check
	if f.bucket == nil {
		return errors.New("parent bucket reference is nil")
	}

	//read message
	messageType, message, err := f.Read()
	if err != nil {
		////if timeout error, do nothing
		//if e, ok := err.(net.Error); ok && e.Timeout() {
		//	//it's a timeout error
		//	//log.Printf("conn id %v read time out, err:%v\n", connId, err.Error())
		//	continue
		//}

		//call relate opt of parent bucket
		//clean the connect obj
		if f.bucket.GetRemote() != nil {
			remoteAddr := f.GetRemoteAddr()
			if remoteAddr != "" {
				f.bucket.GetRemote().DelRemote(remoteAddr)
			}
		}

		//check and call closed cb
		if f.bucket.GetConnClosedCB() != nil {
			f.bucket.GetConnClosedCB()(
				f.bucket.GetRouter().GetName(),
				f.connId,
				f,
				f.GetContext())
		}

		//remove connect from bucket
		f.bucket.CloseConn(f)
		return err
	}

	//update conn active time
	defer func() {
		atomic.StoreInt64(&f.activeTime, time.Now().Unix())
	}()

	////heart beat data check and opt
	//if bytes.Compare(f.router.GetHeartByte(), message) == 0 {
	//	//it's heart beat data
	//	connObj.HeartBeat()
	//	continue
	//}

	//check and call read message cb
	//the err from cb side will skipped!
	cbForReadMessage := f.bucket.GetReadMessageCB()
	if cbForReadMessage != nil {
		f.bucket.GetReadMessageCB()(
			f.bucket.GetRouter().GetName(),
			f.connId,
			f,
			messageType,
			message,
			f.GetContext())
	}
	return err
}

//read data process
//all data from client side
func (f *WSConn) readDataProcess() {
	var (
		err error
		m any = nil
	)
	//catch panic
	defer func() {
		if p := recover(); p != m {
			log.Printf("conn %v panic, err:%v\n", f.connId, p)
		}
	}()

	//loop read data
	for {
		err = f.readOneMessageData()
		if err != nil {
			return
		}
	}
}