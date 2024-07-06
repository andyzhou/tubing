package face

import (
	"github.com/gorilla/websocket"
	"sync"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * dynamic group
 * - one group contain batch dynamic connections
 * - dynamic create and destroy
 * - temp cancelled!!!
 */

//face info
type Group struct {
	connMap map[int64]*websocket.Conn //connId -> Conn (reference)
	history [][]byte //active history message
	welcomeByte []byte
	sync.RWMutex
}

//construct
func NewGroup() *Group {
	this := &Group{
		connMap: map[int64]*websocket.Conn{},
		history: [][]byte{},
		welcomeByte: []byte{},
	}
	return this
}