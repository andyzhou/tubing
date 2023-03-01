package tubing

import (
	"errors"
	"github.com/gin-gonic/gin"
	"sync"
	"tubing/websocket"
)

//global variable
var (
	_tb *Tubing
	_tbOnce sync.Once
)

//info for router cb
type UriRouter struct {
	Uri string
	CBForClosed func(uri string, session string) error
	CBForRead func(uri string, messageType int, message []byte) error
}

//face info
type Tubing struct {
	gin *gin.Engine
	uriMap sync.Map //uri -> iRouter
}

//get single instance
func GetTubing() *Tubing {
	_tbOnce.Do(func() {
		_tb = NewTubing()
	})
	return _tb
}

//construct
func NewTubing(gs ... *gin.Engine) *Tubing {
	this := &Tubing{
		gin: gin.Default(),
		uriMap: sync.Map{},
	}
	if gs != nil && len(gs) > 0 {
		this.gin = gs[0]
	}
	return this
}

//set gin
func (f *Tubing) SetGin(g *gin.Engine) error {
	if g == nil {
		return errors.New("invalid gin engine object")
	}
	f.gin = g
	return nil
}

//register websocket uri
func (f *Tubing) RegisterUri(ur *UriRouter) error {
	//check
	if ur == nil || ur.Uri == "" {
		return errors.New("invalid parameter")
	}
	if f.gin == nil {
		return errors.New("inter gin engine not init yet")
	}
	v, _ := f.GetRouter(ur.Uri)
	if v != nil {
		return errors.New("this uri had registered")
	}

	//init new router
	router := websocket.NewRouter(ur.Uri)
	router.SetCBForClosed(ur.CBForClosed)
	router.SetCBForRead(ur.CBForRead)

	//begin register
	f.gin.GET(ur.Uri, router.Entry)
	f.uriMap.Store(ur.Uri, router)
	return nil
}

//un register websocket uri
func (f *Tubing) UnRegisterUri(uri string) error {
	//check
	if uri == "" {
		return errors.New("invalid parameter")
	}
	//get old
	v, err := f.GetRouter(uri)
	if err != nil || v == nil {
		return errors.New("can't get router by uri")
	}
	f.uriMap.Delete(uri)
	return nil
}

//get router by uri
func (f *Tubing) GetRouter(uri string) (websocket.IRouter, error) {
	//check
	if uri == "" {
		return nil, errors.New("invalid parameter")
	}
	v, ok := f.uriMap.Load(uri)
	if ok && v != nil {
		return v.(websocket.IRouter), nil
	}
	return nil, errors.New("no router for this uri")
}