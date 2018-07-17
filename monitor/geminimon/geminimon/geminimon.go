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
	MySQLEndpoint     string
	MySQLIP           string
	MySQLUsername     string
	MySQLPassword     string
	DatabaseName      string
	ChangeEventsTable string
}

type dbRowType struct {
	time int64
	side uint8
	/*
		0: bid
		1: ask
	*/
	price     float64
	remaining float64
	reason    uint8
	/*
		0: connect
		1: disconnect
		2: place
		3: trade
		4: cancel
		5: initial
	*/
}

var eventID int64
var config configType
var db mysql.Conn
var dbUpdateBuffer bytes.Buffer

//OnModuleStart external calling designation
func OnModuleStart() {
	//unset flag to terminate connection peacefully
	var programRunning = true

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

	rows, _ /*res*/, err := db.Query("SHOW columns FROM " + config.ChangeEventsTable + ";")
	if err != nil {
		panic(err)
	}

	fmt.Println("Database columns:")
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
	websocketEndpoint := config.WebsocketURL + "btcusd" + addParams

	c := connectWebsocket(websocketEndpoint)

	//set true if message handler loop is exited
	hMessageDone := make(chan bool)
	//message handling from websocket
	go func() {
		fmt.Println("Message handling initiated.")

		for {
			//blocks
			_, message, err := c.ReadMessage()

			//TODO: check connection again
			if err != nil {
				fmt.Println("Connection lost: ", err)

				//post an event to the DB for disconnect
				query := fmt.Sprintf(
					"INSERT INTO "+config.ChangeEventsTable+" VALUES (%d, %d, %d, %f, %f, %d);",
					eventID,
					time.Now().UnixNano()/int64(time.Millisecond),
					0,
					0.0,
					0.0,
					1) //reason is disconnect
				fmt.Println(query)
				_, _, err = db.Query(query)
				if err != nil {
					fmt.Println("Error while querying DB for connect event:", err)
				}
				eventID++

				if programRunning {
					c = connectWebsocket(websocketEndpoint)
				} else {
					//if the flag is unset, then the user has manually terminated the program
					break
				}
			} else {
				onWSMessage(message)
			}
		}

		fmt.Println("Message handling terminated.")
		hMessageDone <- true
	}()

	//set true if message handler loop is exited
	hUpdateDBDone := make(chan bool)
	//setup separate timer thread
	go func() {
		fmt.Println("DB updates initiated.")
		for programRunning {
			onUpdateDB()
			time.Sleep(time.Duration(config.DBUpdateMS) * time.Millisecond)
		}
		fmt.Println("DB updates terminated.")
		hUpdateDBDone <- true
	}()

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {
		fmt.Println("Accepting commands...")
		text, err := reader.ReadString('\n')
		rain.CheckError(err)
		text = strings.TrimSpace(text)

		if text == "exit" {
			fmt.Println("Exiting...")

			//set flag that we don't want to reconnect
			programRunning = false

			//cleanly exit monitoring
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println(err)
			}

			//manually close connection here
			c.Close()

			//wait for message handling function to respond and exit
			<-hMessageDone

			<-hUpdateDBDone
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else {
			fmt.Println("Unrecognized command.")
		}
	}

	//conclude
	fmt.Println("Program termianted.")
}

func connectWebsocket(address string) *websocket.Conn {
	fmt.Println("Connecting to websocket endpoint...")

	c, _, err := websocket.DefaultDialer.Dial(address, nil)
	for err != nil {
		time.Sleep(time.Duration(config.ConnectRetryMS) * time.Millisecond)
		c, _, err = websocket.DefaultDialer.Dial(address, nil)
	}

	//everytime we connect, update the db with a reconnect message
	query := fmt.Sprintf(
		"INSERT INTO "+config.ChangeEventsTable+" VALUES (%d, %d, %d, %f, %f, %d);",
		eventID,
		time.Now().UnixNano()/int64(time.Millisecond),
		0,
		0.0,
		0.0,
		0) //reason is connect
	fmt.Println(query)
	_, _, err = db.Query(query)
	if err != nil {
		fmt.Println("Error while querying DB for connect event:", err)
	}
	eventID++

	fmt.Println("Websocket connected.")
	return c
}

func onUpdateDB() {
	//update db
	if dbUpdateBuffer.Len() == 0 {
		return
	}
	queryPart := dbUpdateBuffer.Bytes()

	queryPart[len(queryPart)-1] = ';'
	query := "INSERT INTO " + config.ChangeEventsTable + " VALUES " + string(queryPart)
	fmt.Printf("Updating DB with %d bytes...\r", len(query))
	dbUpdateBuffer.Reset()

	_, _, err := db.Query(query)
	if err != nil {
		fmt.Println("Error while querying DB:", err)
	}
}

func onWSMessage(message []byte) {
	var response map[string]interface{}
	err := json.Unmarshal(message, &response)
	if err != nil {
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
		dbUpdate.time = time.Now().UnixNano() / int64(time.Millisecond)
	} else {
		dbUpdate.time = int64(response["timestampms"].(float64))
	}

	events := response["events"].([]interface{})
	for a := 0; a < len(events); a++ {
		cur := events[a].(map[string]interface{})
		if cur["type"].(string) != "change" {
			continue
		}

		side := cur["side"].(string)
		if side == "bid" {
			dbUpdate.side = 0
		} else if side == "ask" {
			dbUpdate.side = 1
		} else {
			fmt.Println("Unexpected \"side\" value:", side)
			continue
		}

		dbUpdate.price, err = strconv.ParseFloat(cur["price"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"price\" value:", cur["price"].(string))
			continue
		}

		dbUpdate.remaining, err = strconv.ParseFloat(cur["remaining"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"remaining\" value:", cur["remaining"].(string))
			continue
		}

		reason := cur["reason"].(string)
		if reason == "place" {
			dbUpdate.reason = 2
		} else if reason == "trade" {
			dbUpdate.reason = 3
		} else if reason == "cancel" {
			dbUpdate.reason = 4
		} else if reason == "initial" {
			dbUpdate.reason = 5
		} else {
			fmt.Println("Unexpected \"reason\" value:", reason)
			continue
		}

		dbUpdateBuffer.WriteString(fmt.Sprintf("(%d, %d, %d, %f, %f, %d),",
			eventID,
			dbUpdate.time,
			dbUpdate.side,
			dbUpdate.price,
			dbUpdate.remaining,
			dbUpdate.reason))
		eventID++
	}
}
