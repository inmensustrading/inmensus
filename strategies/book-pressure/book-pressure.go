package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

//fuck ur comments
type StrategyServer int

func (t *StrategyServer) OnInputEvent(args *map[string]string, reply *int) error {
	(*args)["test"] = "success"
	return nil
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	//init
	fmt.Println("Starting...")
	fmt.Println("Orderbook pressure strategy, taken from http://eprints.maths.ox.ac.uk/1895/1/Darryl%20Shen%20%28for%20archive%29.pdf")

	//read config into map
	dat, err := ioutil.ReadFile("config.ini")
	checkError(err)
	configStr := string(dat)

	config := make(map[string]string)
	configBytes := bytes.NewBuffer([]byte(configStr))
	configBuffer := make([]byte, 0, configBytes.Len())
	for {
		key := ""
		p := configBytes.Bytes()
		if bytes.Equal(p[:1], []byte(":")) {
			key = string(configBuffer)
			configBuffer = make([]byte, 0, configBytes.Len())
		} else if bytes.Equal(p[:1], []byte("\n")) {
			config[key] = string(configBuffer)
			configBuffer = make([]byte, 0, configBytes.Len())
		}
		configBuffer = append(configBuffer, configBytes.Next(1)...)
	}

	fmt.Println(config)

	//setup listening
	stratServer := new(StrategyServer)
	rpc.Register(stratServer)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", int(config["strategy-port"]))
	if e != nil {
		log.Fatal("Listen error:", e)
	}
	go http.Serve(l, nil)
	fmt.Println("Listening setup.")

	//clean up and conclude
	fmt.Println("Exiting...")
}
