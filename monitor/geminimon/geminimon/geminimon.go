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
	DBUpdateMS        int
	ConnectRetryMS    int
	CheckpointSaveMS  int
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

	c := connectWebsocket(config.WebsocketURL + "btcusd" + addParams)

	defer c.Close()

	//TODO: figure out what this line does
	done := make(chan struct{})

	//message handling from websocket
	go func() {
		defer close(done)
		for {
			//blocks
			_, message, err := c.ReadMessage()

			//TODO: check connection again
			if err != nil {
				fmt.Println("Connection lost: ", err)
				c = connectWebsocket(config.WebsocketURL + "btcusd" + addParams)
			} else {
				onWSMessage(message)
			}
		}
	}()

	//setup separate timer thread
	ticker := time.NewTicker(time.Duration(config.DBUpdateMS) * time.Millisecond)
	tickerQuit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				onUpdateDB()
			case <-tickerQuit:
				ticker.Stop()
				return
			}
		}
	}()

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {
		fmt.Println("Accepting commands...")
		text, err := reader.ReadString('\n')
		rain.CheckError(err)
		text = strings.TrimSpace(text)

		if text == "exit" {
			//cleanly exit monitoring
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

			if err != nil {
				fmt.Println(err)
			}

			//TODO: wait for connection to close here

			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else {
			fmt.Println("Unrecognized command.")
		}
	}

	//conclude
	fmt.Println("Exiting...")
}

func connectWebsocket(address string) *websocket.Conn {
	fmt.Println("Connecting to websocket endpoint...")

	c, _, err := websocket.DefaultDialer.Dial(address, nil)
	for err != nil {
		time.Sleep(time.Duration(config.ConnectRetryMS) * time.Millisecond)
		c, _, err = websocket.DefaultDialer.Dial(address, nil)
	}

	//everytime we connect, update the db with a reconnect message
	query := fmt.Sprintf("INSERT INTO "+config.ChangeEventsTable+" VALUES (%d, \"\", 0, 0, \"connect\");", time.Now().UnixNano()/int64(time.Millisecond))
	fmt.Println(query)
	_, _, err = db.Query(query)
	if err != nil {
		fmt.Println("Error while querying DB for reconnect event:", err)
	}

	fmt.Println("Websocket connected.")
	return c
}

func onUpdateDB() {
	//update db
	queryPart := dbUpdateBuffer.Bytes()
	if dbUpdateBuffer.Len() == 0 {
		return
	}

	queryPart[len(queryPart)-1] = ';'
	query := "INSERT INTO " + config.ChangeEventsTable + " VALUES " + string(queryPart)
	fmt.Printf("Updating DB with %d bytes...\n", len(query))
	dbUpdateBuffer.Reset()

	_, _, err := db.Query(query)
	if err != nil {
		fmt.Println("Error while querying DB:", err)
	}
}

func onWSMessage(message []byte) {
	//fmt.Println(string(message[:]))

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		fmt.Println("Error while unmarshalling message:", err)
		return
	}

	if response["type"].(string) != "update" {
		fmt.Println("Not update type.")
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

		dbUpdateBuffer.WriteString(fmt.Sprintf("(%d, %q, %f, %f, %q),",
			dbUpdate.timestampms,
			dbUpdate.side,
			dbUpdate.price,
			dbUpdate.remaining,
			dbUpdate.reason))
	}
}
