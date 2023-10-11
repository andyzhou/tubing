package face

import (
	"errors"
	"github.com/andyzhou/tubing/define"
	"github.com/gorilla/websocket"
	"sync"
	"sync/atomic"
	"time"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket connect
 * reflect.DeepEqual(x, y)
 */

//web socket connect info
type WSConn struct {
	conn *websocket.Conn //reference
	propMap map[string]interface{}
	activeTime int64
	propLock sync.RWMutex
	connLock sync.RWMutex
}

//construct
func NewWSConn(conn *websocket.Conn) *WSConn {
	this := &WSConn{
		conn: conn,
		propMap: map[string]interface{}{},
	}
	return this
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
		checkRate = define.ServerHeartBeatRate
	}
	now := time.Now().Unix()
	diff := now - f.activeTime
	if diff >= int64(checkRate) {
		return false
	}
	return true
}

//heart beat
func (f *WSConn) HeartBeat() {
	atomic.StoreInt64(&f.activeTime, time.Now().Unix())
}

//verify properties
//if exists return true, or face
func (f *WSConn) VerifyProp(keys ...string) bool {
	//check
	if keys == nil || len(keys) <= 0 {
		return false
	}
	//loop verify
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

//write data
func (f *WSConn) Write(messageType int, data []byte) error {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	if f.conn == nil {
		return errors.New("invalid ws connect")
	}
	atomic.StoreInt64(&f.activeTime, time.Now().Unix())
	return f.conn.WriteMessage(messageType, data)
}

//read data
//return messageType, data, error
func (f *WSConn) Read() (int, []byte, error) {
	if f.conn == nil {
		return 0, nil, errors.New("invalid ws connect")
	}
	return f.conn.ReadMessage()
}

//close conn
func (f *WSConn) CloseWithMessage(message string) error {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, message)
	err := f.conn.WriteMessage(websocket.CloseMessage, msg)
	if err != nil {
		return err
	}
	return f.Close()
}
func (f *WSConn) Close() error {
	f.connLock.Lock()
	defer f.connLock.Unlock()
	return f.conn.Close()
}