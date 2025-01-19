package face

import (
	"errors"
	"log"
	"runtime"
	"sync"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket temp group
 * - dynamic create temp groups
 * - inter ws connections are all references
 * - one router, batch groups
 */

//face info
type Group struct {
	groupId int64
	connMap map[int64]IWSConn //reference conn map
	sync.RWMutex
}

//construct
func NewGroup(groupId int64) *Group {
	this := &Group{
		groupId: groupId,
		connMap: map[int64]IWSConn{},
	}
	return this
}

//clear
func (f *Group) Clear() {
	f.Lock()
	defer f.Unlock()
	f.connMap = map[int64]IWSConn{}
	runtime.GC()
}

//send message to all
func (f *Group) SendMessage(msgType int, msg []byte) error {
	var (
		err error
	)
	//check
	if msgType < 0 || msg == nil || len(msg) <= 0 {
		return errors.New("invalid parameter")
	}

	//cast to all with locker
	f.Lock()
	defer f.Unlock()
	for _, conn := range f.connMap {
		if conn == nil || conn.ConnIsActive() == false {
			continue
		}
		err = conn.Write(msgType, msg)
	}
	return err
}

//quit group
func (f *Group) Quit(connIds ...int64) error {
	//check
	if connIds == nil || len(connIds) <= 0 {
		return errors.New("invalid parameter")
	}

	//remove from map with locker
	f.Lock()
	defer f.Unlock()
	for _, connId := range connIds {
		delete(f.connMap, connId)
	}

	//check and gc opt
	if len(f.connMap) <= 0 {
		f.connMap = map[int64]IWSConn{}
		runtime.GC()
		log.Printf("group:%v, gc opt..\n", f.groupId)
	}
	return nil
}

//join group
func (f *Group) Join(conn IWSConn) error {
	//check
	if conn == nil || conn.GetConnId() <= 0 {
		return errors.New("invalid parameter")
	}

	//sync into map with locker
	f.Lock()
	defer f.Unlock()
	conn.SetGroupId(f.groupId)
	f.connMap[conn.GetConnId()] = conn
	return nil
}