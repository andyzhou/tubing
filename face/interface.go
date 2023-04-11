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
	GetPatternPara(name string) string
	GetManager() IConnManager
	GetCoder() ICoder
}

//interface of manager
type IConnManager interface {
	Close()
	SendMessage(messageType int, message []byte, sessions ... string) error
	CastMessage(messageType int, message []byte, tags ...string) error
	GetConn(session string) (IWSConn, error)
	Accept(session string, conn *websocket.Conn) (IWSConn, error)
	CloseWithMessage(conn *websocket.Conn, message string) error
	CloseConn(sessions ...string) error
}

//interface of connect
type IWSConn interface {
	//adv
	GetTags() []string
	MarkTag(tags ...string) error
	//base
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