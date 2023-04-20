package face

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

/*
 * websocket relate interface define
 */

//interface of router
type IRouter interface {
	Entry(c *gin.Context)
	GetUriPara(name string) string
	GetManager() IConnManager
	GetCoder() ICoder
	SetHeartByte(data []byte) error
	SetMessageType(iType int) error
}

//interface of manager
type IConnManager interface {
	Close()
	SendMessage(message []byte, connIds ... int64) error
	CastMessage(message []byte, tags ...string) error
	HeartBeat(connId int64) error
	SetMessageType(iType int)
	GetConn(connId int64) (IWSConn, error)
	Accept(connId int64, conn *websocket.Conn) (IWSConn, error)
	GenConnId() int64
	GetMaxConnId() int64
	CloseWithMessage(conn *websocket.Conn, message string) error
	CloseConn(connIds ... int64) error
}

//interface of connect
type IWSConn interface {
	//adv
	GetTags() []string
	MarkTag(tags ...string) error
	//base
	HeartBeat()
	Write(messageType int, data []byte) error
	Read() (int, []byte, error)
	CloseWithMessage(message string) error
	Close() error
}

//interface for message en/decoder
type ICoder interface {
	Marshal(contentType int, content proto.Message) ([]byte, error)
	Unmarshal(contentType int, content []byte, req proto.Message) error
}