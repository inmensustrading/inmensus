package geminimon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"../../../common/rain"

	"github.com/gorilla/websocket"
	"github.com/tkanos/gonfig"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native" //native engine
)

type configType struct {
	WebsocketURL      string
	WebsocketParams   []string
	TimerMS           int
	MySQLEndpoint     string
	MySQLIP           string
	MySQLUsername     string
	MySQLPassword     string
	DatabaseName      string
	ChangeEventsTable string
}

type dbRowType struct {
	timestampms int64
	side        string
	price       float64
	remaining   float64
	reason      string
}

var config configType
var db mysql.Conn
var dbUpdateBuffer bytes.Buffer

//OnModuleStart external calling designation
func OnModuleStart() {
	//init
	fmt.Println("Starting...")
	fmt.Println("Gemini Monitor.")

	//read config
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information.")
	}
	fmt.Println("File directory: ", path.Dir(filename))
	err := gonfig.GetConf(path.Dir(filename)+"/conf.json", &config)
	rain.CheckError(err)
	fmt.Println("Configuration: ", config)

	//connect to db
	db = mysql.New("tcp", "", config.MySQLEndpoint+":3306", config.MySQLUsername, config.MySQLPassword, config.DatabaseName)

	err = db.Connect()
	if err != nil {
		panic(err)
	}

	rows, _ /*res*/, err := db.Query("show columns from " + config.ChangeEventsTable)
	if err != nil {
		panic(err)
	}

	fmt.Println("Database table columns:")
	for _, row := range rows {
		for a := 0; a < len(row); a++ {
			fmt.Print(row.Str(a) + " | ")
		}
		fmt.Println()
	}

	//setup websocket listening
	addParams := "?"
	for _, elem := range config.WebsocketParams {
		addParams += elem + "&"
	}
	addParams = strings.TrimSuffix(addParams, "&")
	c, _, err := websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
	for err != nil {
		fmt.Println(err)
		time.Sleep(time.Second)
		c, _, err = websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
	}
	defer c.Close()

	done := make(chan struct{})

	//message handling from websocket
	go func() {
		defer close(done)
		for {
			//I think this blocks
			_, message, err := c.ReadMessage()

			//TODO: check connection again
			if err != nil {
				fmt.Println(err)
				fmt.Println("Retrying connection...")

				c, _, err = websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
				for err != nil {
					fmt.Println(err)
					time.Sleep(time.Second)
					c, _, err = websocket.DefaultDialer.Dial(config.WebsocketURL+"btcusd"+addParams, nil)
				}
			} else {
				onWSMessage(message)
			}
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
	//update db
	queryPart := dbUpdateBuffer.Bytes()
	if dbUpdateBuffer.Len() == 0 {
		//fmt.Println("no updates")
		//fmt.Println()
		return
	}

	queryPart[len(queryPart)-1] = ';'
	query := "insert into " + config.ChangeEventsTable + " values " + string(queryPart)
	//fmt.Println(query)
	//fmt.Println()
	dbUpdateBuffer.Reset()

	_, _, err := db.Query(query)
	if err != nil {
		panic(err)
	}
}

func onWSMessage(message []byte) {
	//fmt.Println(string(message[:]))

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		panic(err)
	}

	if response["type"].(string) != "update" {
		fmt.Println("not update type")
		return
	}

	dbUpdate := dbRowType{}
	if response["timestampms"] == nil {
		//probably initial event
		//set timestamp to be current time
		dbUpdate.timestampms = time.Now().UnixNano() / int64(time.Millisecond)
	} else {
		dbUpdate.timestampms = int64(response["timestampms"].(float64))
	}

	events := response["events"].([]interface{})
	for a := 0; a < len(events); a++ {
		cur := events[a].(map[string]interface{})
		if cur["type"].(string) != "change" {
			continue
		}

		dbUpdate.side = cur["side"].(string)
		dbUpdate.price, _ = strconv.ParseFloat(cur["price"].(string), 64)
		dbUpdate.remaining, _ = strconv.ParseFloat(cur["remaining"].(string), 64)
		dbUpdate.reason = cur["reason"].(string)

		//fmt.Println(dbUpdate)
		dbUpdateBuffer.WriteString(fmt.Sprintf("(%d, %q, %f, %f, %q),",
			dbUpdate.timestampms,
			dbUpdate.side,
			dbUpdate.price,
			dbUpdate.remaining,
			dbUpdate.reason))
	}
}
