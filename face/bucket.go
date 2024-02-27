package face

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"sync"

	"github.com/andyzhou/tinylib/queue"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * connect bucket
 * - batch buckets contain all ws connects
 * - one son worker contain batch connect
 */

//face info
type Bucket struct {
	router IRouter //reference from outside
	worker *queue.Worker
	cbForReadMessage func(string, int64, int, []byte) error
	cbForConnClosed func(string, int64, ...*gin.Context) error
	sync.RWMutex
}

//construct
func NewBucket(router IRouter) *Bucket {
	this := &Bucket{
		router: router,
		worker: queue.NewWorker(),
	}
	this.interInit()
	return this
}

//quit
func (f *Bucket) Quit() {
	f.worker.Quit()
}

//////////////////
//api for cb func
//////////////////

//set cb for read message, step-1-1
func (f *Bucket) SetCBForReadMessage(cb func(string, int64, int, []byte) error)  {
	if cb == nil {
		return
	}
	f.cbForReadMessage = cb
}

//set cb for conn closed, step-1-2
func (f *Bucket) SetCBForConnClosed(cb func(string, int64, ...*gin.Context) error) {
	if cb == nil {
		return
	}
	f.cbForConnClosed = cb
}

////set cb for bind connects read opt, step-2
//func (f *Bucket) SetCBForReadOpt(cb func(int32, ...interface{}) error) {
//	if cb == nil {
//		return
//	}
//	f.worker.SetCBForBindObjTickerOpt(cb)
//}

//create batch son workers, step-3
func (f *Bucket) CreateSonWorkers(num int, tickRates ...float64) error {
	//check
	if num <= 0 {
		return errors.New("invalid parameter")
	}
	//create batch son workers
	err := f.worker.CreateWorkers(num, tickRates...)
	return err
}

///////////////////////
//api for send message
///////////////////////

//send sync message
func (f *Bucket) SendMessage(data interface{}, connectIds ...int64) error {
	//check
	if data == nil || connectIds == nil {
		return errors.New("invalid parameter")
	}

	//send to target workers
	_, err := f.worker.SendData(data, connectIds)
	return err
}

func (f *Bucket) SendMessageToWorker(data interface{}, workerId int32) error {
	//check
	if data == nil || workerId <= 0 {
		return errors.New("invalid parameter")
	}

	//get target worker
	sonWorker, err := f.worker.GetWorker(workerId)
	if err != nil || sonWorker == nil {
		return err
	}

	//send to target worker
	_, err = sonWorker.SendData(data)
	return err
}

//cast async message to all son workers
func (f *Bucket) CastMessage(data interface{}) error {
	//check
	if data == nil {
		return errors.New("invalid parameter")
	}

	//cast to all
	err := f.worker.CastData(data)
	return err
}

//////////////////
//api for connect
//////////////////

//get connect by id
func (f *Bucket) GetConnect(connId int64) (*WSConn, error) {
	//check
	if connId <= 0 {
		return nil, errors.New("invalid parameter")
	}

	//get target son worker
	sonWorker, err := f.worker.GetTargetWorker(connId)
	if err != nil {
		return nil, err
	}
	if sonWorker == nil {
		return nil, errors.New("can't get son worker")
	}

	//get connect
	obj, subErr := sonWorker.GetBindObj(connId)
	if subErr != nil || obj == nil {
		return nil, subErr
	}
	conn, ok := obj.(*WSConn)
	if !ok || conn == nil {
		return nil, errors.New("invalid obj type")
	}
	return conn, nil
}

//remove connect
func (f *Bucket) RemoveConnect(connId int64) error {
	//check
	if connId <= 0 {
		return errors.New("invalid parameter")
	}

	//get target son worker
	sonWorker, err := f.worker.GetTargetWorker(connId)
	if err != nil {
		return err
	}
	if sonWorker == nil {
		return errors.New("can't get son worker")
	}

	//remove connect from target son worker
	err = sonWorker.RemoveBindObj(connId)
	return err
}

//add new connect
func (f *Bucket) AddConnect(conn *WSConn) error {
	//check
	if conn == nil {
		return errors.New("invalid parameter")
	}

	//get target son worker
	connId := conn.GetConnId()
	sonWorker, err := f.worker.GetTargetWorker(connId)
	if err != nil {
		return err
	}
	if sonWorker == nil {
		return errors.New("can't get son worker")
	}

	//save new connect into target son worker
	err = sonWorker.UpdateBindObj(connId, conn)
	return err
}

///////////////
//private func
///////////////

//cb for read connect data
func (f *Bucket) cbForReadConnData(
	workerId int32,
	connMaps ...interface{}) error {
	var (
		messageType int
		message []byte
		err error
	)
	//check
	if workerId <= 0 || connMaps == nil ||
		len(connMaps) <= 0 {
		return errors.New("invalid parameter")
	}
	mapVal := connMaps[0]
	if mapVal == nil {
		return errors.New("invalid conn map data")
	}
	connMap, ok := mapVal.(map[int64]interface{})
	if !ok || connMap == nil || len(connMap) <= 0 {
		return errors.New("no any conn map data")
	}

	//loop read connect data
	for connId, conn := range connMap {
		//check connect
		if connId <= 0 || conn == nil {
			continue
		}
		connObj, subOk := conn.(*WSConn)
		if !subOk || connObj == nil {
			continue
		}

		//read message
		messageType, message, err = connObj.Read()
		if err != nil {
			//close connect and remove it
			connObj.Close()

			//check and call closed cb
			if f.cbForConnClosed != nil {
				f.cbForConnClosed(f.router.GetName(), connId)
			}

			//remove from manager
			f.router.GetManager().CloseConn(connId)
			continue
		}
		if bytes.Compare(f.router.GetHeartByte(), message) == 0 {
			//it's heart beat data
			connObj.HeartBeat()
			continue
		}

		//check and call read message cb
		if f.cbForReadMessage != nil {
			f.cbForReadMessage(f.router.GetName(), connId, messageType, message)
		}
	}
	return err
}

//batch son workers
func (f *Bucket) createSonWorkers() {
	buckets := f.router.GetConf().Buckets
	readRate := f.router.GetConf().ReadByteRate
	for i := 0; i < buckets; i++ {
		f.CreateSonWorkers(i, readRate)
	}
}

//inter init
func (f *Bucket) interInit() {
	//set cb for read connect data
	f.worker.SetCBForBindObjTickerOpt(f.cbForReadConnData)

	//create batch son workers
	f.createSonWorkers()
}