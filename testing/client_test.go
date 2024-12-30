package testing

import (
	"fmt"
	"github.com/andyzhou/tubing"
	"sync"
	"testing"
	"time"
)

const (
	Host = "127.0.0.1"
	Port = 8090
	Uri  = "/ws"
)

var (
	clients = map[int64]*tubing.OneWSClient{}
	client *tubing.WebSocketClient
	//init conn para
	para = &tubing.WebSocketConnPara{
		Host: Host,
		Port: Port,
		Uri: Uri,
		CBForReadMessage: readMessage,
	}
)

//init
func init() {
	client = tubing.NewWebSocketClient()
}

//read message
func readMessage(connId int64, messageType int, message []byte) error {
	//log.Printf("readMessage, session:%v, message:%v\n",
	//	message.MessageType,
	//	string(message.Message))
	return nil
}

//write message
func sendMessage(c *tubing.OneWSClient, wg *sync.WaitGroup, b *testing.B) {
	defer (*wg).Done()
	if c.IsClosed() {
		return
	}
	message := []byte(fmt.Sprintf("hello %v", time.Now().Unix()))
	err := c.SendMessage(message)
	if err != nil {
		b.Errorf("sendMessage, err:%v\n", err.Error())
		return
	}
}

//create ws client
func createWsClient() (*tubing.OneWSClient, error) {
	return client.CreateClient(para)
}

//test api
func TestClient(t *testing.T) {
	subClient, err := createWsClient()
	if err != nil {
		t.Errorf("test create failed, err:%v\n", err.Error())
	}
	t.Logf("test create succeed, conn id:%v\n", subClient.GetConnId())
}

//benchmark api
func BenchmarkClient(b *testing.B) {
	var (
		wg = sync.WaitGroup{}
	)
	succeed := 0
	failed := 0
	wsArr := make([]*tubing.OneWSClient, 0)
	for i := 0; i < b.N; i++ {
		ws, err := createWsClient()
		if err != nil {
			failed++
			break
		}

		//send message
		wg.Add(1)
		sendMessage(ws, &wg, b)
		wsArr = append(wsArr, ws)
		succeed++
	}
	b.Logf("benchmark create done, N:%v, succeed:%v, failed:%v\n",
		b.N, succeed, failed)

	wg.Wait()
	b.Logf("benchmark, all done, clean up\n")

	//close connect
	for _, v := range wsArr {
		v.Quit()
	}
	wsArr = []*tubing.OneWSClient{}
	return
}
