package main

import (
	"fmt"
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
	for {
		message := []byte(fmt.Sprintf("hello %v", time.Now().Unix()))
		err := c.SendMessage(message)
		if err != nil {
			log.Printf("sendMessage, err:%v\n", err.Error())
		}
		time.Sleep(time.Second * 2)
	}
}

//send heart beat
func sendHeartBeat(c *tubing.OneWSClient) {
	ticker := time.NewTicker(time.Second * 5)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <- ticker.C:
			{
				c.SendMessage([]byte(define.MessageBodyOfHeartBeat))
			}
		}
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
	go sendHeartBeat(oneClient)

	//auto close
	sf := func() {
		c.Close()
	}
	time.AfterFunc(time.Second * 5, sf)

	wg.Add(1)
	log.Println("client run..")
	wg.Wait()
}
