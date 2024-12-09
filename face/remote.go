package face

import (
	"errors"
	"github.com/andyzhou/tubing/define"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

/*
 * remote data face
 * - storage remote addr and conn id in run env
 * - work for manager
 */

//face info
type Remote struct {
	remoteAddrMap map[string]int64 //remoteAddr -> connectId
	sync.RWMutex
}

//construct
func NewRemote() *Remote {
	this := &Remote{
		remoteAddrMap: map[string]int64{},
	}
	return this
}

//clean up
func (f *Remote) Cleanup() {
	//clean with locker
	f.Lock()
	defer f.Unlock()
	f.remoteAddrMap = map[string]int64{}
	runtime.GC()
}

//del remote
func (f *Remote) DelRemote(addr string) error {
	//check
	if addr == "" {
		return errors.New("invalid parameter")
	}

	//del with locker
	f.Lock()
	defer f.Unlock()
	delete(f.remoteAddrMap, addr)

	//check and reset
	remoteAddrLen := len(f.remoteAddrMap)
	if remoteAddrLen <= 0 {
		f.remoteAddrMap = map[string]int64{}
	}

	//cal gc rate
	rand.Seed(time.Now().UnixNano())
	gcRate := rand.Intn(100)
	if gcRate <= define.GcOptRate || remoteAddrLen <= 0 {
		runtime.GC()
		log.Printf("remote.DelRemote, gcRate:%v, remoteAddrLen:%v, gc opt...\n",
			gcRate, remoteAddrLen)
	}
	return nil
}

//get conn id by remote addr
func (f *Remote) GetRemote(addr string) (int64, error) {
	//check
	if addr == "" {
		return 0, errors.New("invalid parameter")
	}

	//get with locker
	f.Lock()
	defer f.Unlock()
	v, ok := f.remoteAddrMap[addr]
	if ok && v >= 0 {
		return v, nil
	}
	return 0, nil
}

//add new remote
func (f *Remote) AddRemote(addr string, connId int64) error {
	//check
	if addr == "" || connId <= 0 {
		return errors.New("invalid parameter")
	}

	//add with locker
	f.Lock()
	defer f.Unlock()
	f.remoteAddrMap[addr] = connId
	return nil
}
