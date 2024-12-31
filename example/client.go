package main

import (
	"fmt"
	"github.com/andyzhou/tubing"
	"github.com/andyzhou/tubing/define"
	"log"
	"sync"
	"time"
)

//cb for read message from server side
func cbForReadMessage(connId int64, messageType int, message []byte) error {
	//log.Printf("cbForReadMessage, connId:%v, messageType:%v, message:%v\n", connId, messageType, message)
	return nil
}

//send message
func sendMessage(c *tubing.OneWSClient) {
	for {
		if c.IsClosed() {
			time.Sleep(time.Second)
			break
		}
		message := []byte(fmt.Sprintf("hello %v", time.Now().Unix()))
		err := c.SendMessage(message)
		if err != nil {
			log.Printf("sendMessage, err:%v\n", err)
		}
		time.Sleep(time.Second/10)
	}
	log.Printf("send message done!\n")
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
		clients = 128
	)

	//init conn para
	para := &tubing.WebSocketConnPara{
		Host: "127.0.0.1",
		Port: 8090,
		Uri: "/ws",
		CBForReadMessage: cbForReadMessage,
	}

	//init client
	c := tubing.NewWebSocketClient()
	c.SetCBForReadMessage(cbForReadMessage)

	//create batch sub clients
	for i := 0; i < clients; i++ {
		oneClient, err := c.CreateClient(para)
		if err != nil {
			panic(any(err))
		}
		//spawn send message process
		go sendMessage(oneClient)
		//go sendHeartBeat(oneClient)
	}

	//auto close after 20 seconds
	//sf := func() {
	//	c.Close()
	//}
	//time.AfterFunc(time.Second * 20, sf)

	wg.Add(1)
	log.Println("client run..")
	wg.Wait()
}
