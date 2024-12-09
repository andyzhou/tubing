package face

import (
	"errors"
	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * one websocket connect
 * reflect.DeepEqual(x, y)
 */

//web socket connect info
type WSConn struct {
	connId     int64 //connect id
	ownerId    int64
	ctx        *gin.Context    //reference
	conn       *websocket.Conn //reference
	propMap    map[string]interface{}
	tagMap     map[string]bool
	remoteAddr string
	activeTime int64
	tagLock    sync.RWMutex
	propLock   sync.RWMutex
}

//construct
func NewWSConn(conn *websocket.Conn, connId int64, ctx *gin.Context) *WSConn {
	this := &WSConn{
		connId: connId,
		conn: conn,
		ctx: ctx,
		propMap: map[string]interface{}{},
		tagMap: map[string]bool{},
	}
	return this
}

//close conn
func (f *WSConn) CloseWithMessage(message string) error {
	//close with locker
	//f.connLock.Lock()
	//defer f.connLock.Unlock()
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	f.conn.WriteMessage(websocket.CloseMessage, msg)
	return f.Close()
}
func (f *WSConn) Close() error {
	//close with locker
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

//get connect id
func (f *WSConn) GetConnId() int64 {
	return f.connId
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
	//f.connLock.Lock()
	defer func() {
		atomic.StoreInt64(&f.activeTime, time.Now().Unix())
		//f.connLock.Unlock()
	}()
	return f.conn.WriteMessage(messageType, data)
}

//read data
//return messageType, data, error
func (f *WSConn) Read() (int, []byte, error) {
	//check
	if f.conn == nil {
		return 0, nil, errors.New("invalid ws connect")
	}

	//read message with locker
	//f.connLock.Lock()
	//defer f.connLock.Unlock()
	return f.conn.ReadMessage()
}