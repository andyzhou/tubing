package main

import (
	"github.com/andyzhou/tubing"
	"github.com/andyzhou/tubing/define"
	"log"
	"sync"
	"time"
)

//read message
func readMessage(message *tubing.WebSocketMessage) error {
	log.Printf("readMessage, session:%v, message:%v\n", message.MessageType, string(message.Message))
	return nil
}

//send message
func sendMessage(c *tubing.OneWSClient) {
	messageType := define.MessageTypeOfOctet
	message := []byte("hello")
	for {
		c.SendMessage(messageType, message)
		time.Sleep(time.Second)
	}
}

func main() {
	var (
		wg sync.WaitGroup
	)

	//init conn para
	para := &tubing.WebSocketConnPara{
		Host: "127.0.0.1",
		Port: 8090,
		Uri: "/ws",
		QueryPara: map[string]interface{}{
			"session":"1234",
		},
		CBForReadMessage: readMessage,
	}

	//init client
	c := tubing.NewWebSocketClient()
	oneClient, err := c.CreateClient(para)
	if err != nil {
		log.Println("create client failed, err:", err.Error())
		return
	}

	//spawn send message process
	go sendMessage(oneClient)
	wg.Add(1)
	log.Println("client run..")
	wg.Wait()
}
