package face

import (
	"github.com/andyzhou/tubing/define"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * websocket relate interface define
 */

//interface of router
//one route uri, one router
type IRouter interface {
	Quit()
	Entry(c *gin.Context)

	//get opt
	GetUriPara(name string, ctx *gin.Context) string
	GetManager() IManager
	GetCoder() ICoder
	GetName() string
	GetConf() *RouterCfg
	GetHeartByte() []byte
	GetRouterCfg() *RouterCfg

	//set opt
	SetHeartByte(data []byte) error
	SetMessageType(iType int) error
}

//interface of manager
//one router, one manager
type IManager interface {
	//for message
	Quit()
	SendMessage(para *define.SendMsgPara) error
	SetMessageType(iType int)

	//for bucket
	GetBuckets() map[int]IBucket
	GetBucket(id int) (IBucket, error)

	//for connect
	CloseWithMessage(conn *websocket.Conn, message string) error
	CloseConnect(connIds ...int64) error

	GetConn(connId int64) (IWSConn, error)
	Accept(connId int64, conn *websocket.Conn, ctx *gin.Context) (IWSConn, error)
	GenConnId() int64

	//for group
	RemoveGroup(groupId int64) error
	GetGroup(groupId int64) (IGroup, error)
	CreateGroup(groupId int64) error

	//cb opt
	SetCBForReadMessage(cb func(string, int64, IWSConn, int, []byte, *gin.Context) error)
	SetCBForConnClosed(cb func(string, int64, IWSConn, *gin.Context) error)

	//for base
	GetRouter() IRouter
	GetRemote() IRemote
}

//interface of remote
//one manager, one remote
type IRemote interface {
	Cleanup()
	DelRemote(addr string) error
	GetRemote(addr string) (int64, error)
	AddRemote(addr string, connId int64) error
}

//interface of bucket
//one manager, batch buckets
type IBucket interface {
	//other opt
	Quit()
	GetRouter() IRouter
	GetRemote() IRemote
	GetReadMessageCB() func(string, int64, IWSConn, int, []byte, *gin.Context) error
	GetConnClosedCB() func(string, int64, IWSConn, *gin.Context) error

	//opt for message
	SendMessage(para *define.SendMsgPara) error

	//opt for connect
	GetAllConnect() map[int64]IWSConn
	GetConnect(connId int64) (IWSConn, error)
	CloseConnectByIds(connIds ...int64) (map[int64]string, error)
	CloseConn(conn IWSConn) error
	AddConnect(conn IWSConn) error

	//opt for cb func
	SetCBForReadMessage(cb func(string, int64, IWSConn, int, []byte, *gin.Context) error)
	SetCBForConnClosed(cb func(string, int64, IWSConn, *gin.Context) error)
	SetMsgType(msgType int)
}

//interface of group
//one manager, batch groups
type IGroup interface {
	Clear()
	SendMessage(msgType int, msg []byte) error
	Quit(connIds ...int64) error
	Join(conn IWSConn) error
}

//interface of ws connect
//one bucket, batch connects
type IWSConn interface {
	//adv
	GetConnId() int64
	GetConnect() *websocket.Conn
	GetContext() *gin.Context
	GetRemoteAddr() string
	ConnIsActive(checkRates ...int) bool

	//group id
	GetGroupId() int64
	SetGroupId(groupId int64)

	//tags
	GetTags() map[string]bool
	RemoveTags(tags ...string) error
	MarkTags(tags ...string) error

	//owner id
	GetOwnerId() int64
	SetOwnerId(ownerId int64) error

	//property
	VerifyProp(keys ...string) bool
	DelProp(key string) error
	GetAllProp() map[string]interface{}
	GetProp(key string) (interface{}, error)
	SetProp(key string, val interface{}) error

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