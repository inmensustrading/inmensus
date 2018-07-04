package geminimonitor

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"rain"

	"github.com/gorilla/websocket"
	"github.com/tkanos/gonfig"
)

type configType struct {
	WebsocketURL    string
	WebsocketParams []string
	TimerMS         int
	DBIP            string
}

//OnModuleStart external calling designation
func OnModuleStart(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Gemini Monitor.")

	//read config
	config := configType{}
	err := gonfig.GetConf(configPath, &config)
	rain.CheckError(err)
	fmt.Println("Configuration: ", config)

	//setup websocket listening
	addParams := "?"
	for _, elem := range config.WebsocketParams {
		addParams += elem + "&"
	}
	addParams = strings.TrimSuffix(addParams, "&")
	c, _, err := websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			onWSMessage(message)
		}
	}()

	ticker := time.NewTicker(time.Duration(config.TimerMS) * time.Millisecond)
	tickerQuit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				onTimerCall()
			case <-tickerQuit:
				ticker.Stop()
				return
			}
		}
	}()

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {
		fmt.Print("Enter command: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("error reading command:", err)
		}
		text = strings.TrimSpace(text)

		if text == "exit" {
			//cleanly exit monitoring
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println("Error while writing websocket close message:", err)
				return
			}
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else {
			fmt.Println("Unrecognized command.")
		}
	}

	//clean up and conclude
	fmt.Println("Exiting...")
	time.Sleep(time.Second)
}

func onTimerCall() {
}

func onWSMessage(message []byte) {
	fmt.Println(string(message[:]))
}