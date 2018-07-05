package geminimon

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"../../../common/rain"

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
func OnModuleStart() {
	//init
	fmt.Println("Starting...")
	fmt.Println("Gemini Monitor.")

	//read config
	config := configType{}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information.")
	}
	fmt.Println("File directory: ", path.Dir(filename))
	err := gonfig.GetConf(path.Dir(filename)+"/conf.json", &config)
	rain.CheckError(err)
	fmt.Println("Configuration: ", config)

	//setup websocket listening
	addParams := "?"
	for _, elem := range config.WebsocketParams {
		addParams += elem + "&"
	}
	addParams = strings.TrimSuffix(addParams, "&")
	c, _, err := websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
	rain.CheckError(err)
	defer c.Close()

	done := make(chan struct{})

	//message handling from websocket
	go func() {
		defer close(done)
		for {
			//I think this blocks
			_, message, err := c.ReadMessage()
			rain.CheckError(err)

			onWSMessage(message)
		}
	}()

	//setup separate timer thread
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
		rain.CheckError(err)
		text = strings.TrimSpace(text)

		if text == "exit" {
			//cleanly exit monitoring
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			rain.CheckError(err)
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else {
			fmt.Println("Unrecognized command.")
		}
	}

	//conclude
	fmt.Println("Exiting...")

	//wait for websocket to end
	//TODO: make this better
	time.Sleep(time.Second)
}

func onTimerCall() {

}

func onWSMessage(message []byte) {
	fmt.Println(string(message[:]))
}
