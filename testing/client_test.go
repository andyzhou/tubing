package testing

import (
	"github.com/andyzhou/tubing"
	"testing"
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
func readMessage(message *tubing.WebSocketMessage) error {
	//log.Printf("readMessage, session:%v, message:%v\n",
	//	message.MessageType,
	//	string(message.Message))
	return nil
}

//create ws client
func createWsClient() (*tubing.OneWSClient, error) {
	return client.CreateClient(para)
}

//test api
func TestCreate(t *testing.T) {
	subClient, err := createWsClient()
	if err != nil {
		t.Errorf("test create failed, err:%v\n", err.Error())
	}
	t.Logf("test create succeed, conn id:%v\n", subClient.GetConnId())
}

//benchmark api
func BenchmarkCreate(b *testing.B) {
	succeed := 0
	failed := 0
	wsArr := make([]*tubing.OneWSClient, 0)
	for i := 0; i < b.N; i++ {
		ws, err := createWsClient()
		if err != nil {
			failed++
			break
		}
		wsArr = append(wsArr, ws)
		succeed++
	}
	b.Logf("benchmark create done, N:%v, succeed:%v, failed:%v\n",
		b.N, succeed, failed)
	//close connect
	for _, v := range wsArr {
		v.Quit()
	}
	wsArr = []*tubing.OneWSClient{}
	return
}
