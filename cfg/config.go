package cfg

import "sync"

/*
 * app config
 */

//global variable
var (
	appConf *AppConf
	appConfOnce sync.Once
)

//config info
type (
	BasicConf struct {
	}

	HttpConf struct {
		WsRootUri string
	}

	AppConf struct {
		Basic *BasicConf
		Http *HttpConf
	}
)

//get single instance
func GetAppConf() *AppConf {
	appConfOnce.Do(func() {
		appConf = NewAppConf()
	})
	return appConf
}

//construct
func NewAppConf() *AppConf {
	this := &AppConf{
		Basic: &BasicConf{},
		Http: &HttpConf{},
	}
	this.loadConfig()
	return this
}

func (c *AppConf) loadConfig() {

}