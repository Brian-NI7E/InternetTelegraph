package main

import (
    "encoding/json"
    "fmt"
    "os"
    "bytes"
    "strconv"
    "time"

    "github.com/stianeikeland/go-rpio"
    "golang.org/x/net/websocket"
)



const keyPinBCM     = 07    // keyPinNumber        = 26
const spkrPinBCM    = 10    // spkrPinNumber       = 19

// operation constants
const (
        mainLoopMs      = 10
        mainLoop1Sec    = 1000 / mainLoopMs
        mainLoop5Sec    = mainLoop1Sec * 5
        mainLoop30Sec   = mainLoop1Sec * 30
    )



var (
    buildVersion    string  // go build -ldflags "-X main.buildVersion=<version info> ...
    toneState       tone
    key             morseKey

)



// struction to hold application configuration information
type Config struct {
    Channel string
    Server  string
    Port    string
    Gpio    bool
}


type morseKey struct {
    state   string
    keyPin  rpio.Pin
}



type tone struct {
    spkrPin rpio.Pin
    command rpio.State
}


type socketClient struct {
    ip          string
    port        string
    channel     string
    status      string
    url         string
    redialCount int
    conn        *websocket.Conn
}


const(
    SC_NOT_STARTED  = "not started"
    SC_DISCONNECTED = "disconnected"
    SC_CONNECTED    = "connected"
    SC_RECONNECTING = "reconnecting"
    )



func getConfiguration() (Config) {

    // allow for future feature of using an OS environment variable, for not, we hardcode it
    os.Setenv("TELEGRAPH_CONFIG_PATH", "config.json")
    file, _ := os.Open(os.Getenv("TELEGRAPH_CONFIG_PATH"))
    decoder := json.NewDecoder(file)

    // allow for future feature of using alternate input methods
    config  := Config{Gpio: true}

    // read application configuration from the TELEGRAPH_CONFIG_PATH file
    err     := decoder.Decode(&config)
    if err != nil {
        fmt.Println("Error reading config.json: ", err)
        config.Channel  = "lobby"
        config.Server   = "morse.autodidacts.io"
        config.Port     = "8000"
    }

    fmt.Println("configuration: ", config)

    return config
}


func initializeRpio() int {

    var result int  = 0

    if rpioErr := rpio.Open() ; rpioErr != nil {
        fmt.Println("Error initializing RPIO: ", rpioErr)
        result = -1
    } else {
        fmt.Println("RPIO Open success")
    }

    // Initialize the rpio input and output pins
    key.keyPin          = rpio.Pin(keyPinBCM)
    key.keyPin.Input()
    key.keyPin.PullUp()

    return  result
}


func intitializeToneState(ts tone) tone {
    ts.spkrPin = rpio.Pin(10)  // spkrPinBCM)
    ts.spkrPin.Output()
    ts.command  = rpio.Low      // turn off the tone

    return ts
}


func initializeSocketClient(config Config) socketClient {
    // Init socketClient & dial websocket
    sc := socketClient{ ip: config.Server, 
                        port: config.Port, 
                        channel: config.Channel,
                        status: SC_NOT_STARTED,
                        redialCount: 0}
    var url bytes.Buffer

    url.WriteString("ws://")
    url.WriteString(sc.ip)
    url.WriteString(":")
    url.WriteString(sc.port)
    url.WriteString("/channel/")
    url.WriteString(sc.channel)

    sc.url  = url.String()

    return sc
}


func (sc *socketClient) dial(c chan rpio.State) {
    fmt.Println("Dialing ",  sc.url)

    conn, err := websocket.Dial(sc.url, "", "http://localhost")
    if err == nil {
        sc.conn = conn
        sc.status = SC_CONNECTED
        fmt.Println("sc.status = " + sc.status)
        fmt.Print("sc.conn dial: ")
        fmt.Println(sc.conn)
        // playMorse(".-. . .- -.. -.--", c)
        // playMorse(".--. --- ... - ..... ----. ----.", c)
        // playMorseElements(".--. --- ... - ..... ----. ----.", c)
    } else {
        fmt.Println("Error connecting to '" + sc.url + "': " + err.Error())
    }
}



func (sc    *socketClient) sendMsg(msg string) {
    fmt.Print("Sending: ")
    fmt.Println(msg)
    fmt.Println("---------------")
    sendErr := websocket.Message.Send(sc.conn, msg)
    if sendErr != nil {
        sc.status = SC_DISCONNECTED
        fmt.Print("sc.conn send: ")
        fmt.Println(sc.conn)
        fmt.Println("Could not send message:")
        fmt.Println(sendErr.Error())
    }
}




func (sc *socketClient) listen(c chan rpio.State) {
    fmt.Println("Client listening...")
    var msg string
    for sc.status == SC_CONNECTED {
        err := websocket.Message.Receive(sc.conn, &msg)
        if err == nil {
            // message received - process it
            fmt.Println("received from server: ", msg, "msg[:1]: ", msg[:1])
            fmt.Println("---------------")
            // TODO use key down count to allow logical ORing of multiple keys
            if msg[:1] == "0" {
                c <- rpio.Low
            } else if msg[:1] == "1" {
                c <- rpio.High
            } else {
                // do nothing
            }

            // sc.onMessage(msg)
        } else if 2 > sc.redialCount {
            sc.status = SC_DISCONNECTED
            fmt.Println("Websocket error on Message.Receive(): " + err.Error())
        }
    }
    fmt.Println("FATAL ERROR: socket client not connected!")
}




func (t *tone) control(c chan rpio.State) {
    // TODO add timeout to key down messages from server
    // TOOD allow local key down to run forever ???
    var command rpio.State
    for {
        command = <-c
        t.spkrPin.Write(command)
    }
}




func playMorseElements(message string, c chan rpio.State) {
    // TODO allow config.json file to configure the speed
    // TODO allow optional interruption of message
    var (
            sound           rpio.State
            elementLength   time.Duration
        )
    const WPM20 = 1200/20   // at 20 WPM a dit is 60 ms
    const WPM13 = 1200/13   // at 13 WPM a dit is 92 ms
    ditTime := time.Duration(WPM13) * time.Millisecond

    for i := 0; i < len(message); i++ {
        sound           = rpio.High
        elementLength   = 1

        if '-' == message[i] {
            elementLength   = 3
        } else if ' ' == message[i] {
            sound           = rpio.Low
        } else if '.' != message[i] {
            elementLength   = 0
            sound           = rpio.Low
        }

        c <- sound              // start sounding the element
        time.Sleep(elementLength * ditTime)    
        c <- rpio.Low           // stop sounding the element
        time.Sleep(ditTime)     // add space between elements

    }
}
func playMorse(message string, c chan rpio.State) {
    // TODO allow config.json file to configure the speed
    speed := time.Duration(50)
    for i := 0; i < len(message); i++ {
        switch message[i] {
            case 46: // == "."
            c <- rpio.High
            time.Sleep(speed * time.Millisecond)
            c <- rpio.Low
            time.Sleep(speed * time.Millisecond)
            case 45: // == "-"
            c <- rpio.High
            time.Sleep(3 * speed * time.Millisecond)
            c <- rpio.Low
            time.Sleep(speed * time.Millisecond)
            case 32: // == " "
            time.Sleep(3 * speed * time.Millisecond)
        default:
            c <- rpio.Low       // turn off tone // Do nothing...
        }
    }
}





func microseconds() int64 {
    t := time.Now().UnixNano()
    us := t / int64(time.Microsecond)
    return us
}






func main() {
    fmt.Println("internet-telegraph starting - version", buildVersion)

    var keyValue rpio.State
    var lastKeyValue rpio.State  = rpio.High    // will a pull up un press = High
    var keyToken string = "0"                   // default to no tone
    var loopCount   int = 0


    config  := getConfiguration()

    initializeRpio()
    defer rpio.Close()      // close and cleanup rpio when main closes

    toneState   = intitializeToneState(toneState)
    toneControl := make(chan rpio.State)
    go toneState.control( toneControl)

    serverSocket  := initializeSocketClient(config)
    fmt.Println("SocketClient: ", serverSocket)   // TODO: remove this line


    serverSocket.dial( toneControl)       // establish connection to server
    if SC_CONNECTED == serverSocket.status {
        playMorseElements(".--. --- ... - ..... ----. ----.", toneControl)
    }

    go serverSocket.listen(toneControl)

    for {
        // TODO organize main loop as a scheduler

        // This section of the main loop runs everytime
        // This section of the main loop runs every 5 seconds
        // This section of the main loop runs every 30 seconds

        if SC_CONNECTED != serverSocket.status {
            // attempt to redial
            serverSocket.redialCount++
            serverSocket.status         = SC_RECONNECTING
            if 3 > serverSocket.redialCount {
                // attempt immediate redails
                serverSocket.dial( toneControl)       // reestablish connection
            } else {
                // TODO redial at slower intervals
                if 3 == serverSocket.redialCount % 500 {
                    fmt.Println("Redialing in 5 seconds...")
                    serverSocket.dial( toneControl)       // reestablish connection
                    if SC_CONNECTED == serverSocket.status {
                        // connection restored after prolonged disconnect
                        playMorseElements("..", toneControl)
                    }
                }
            }
            if SC_CONNECTED == serverSocket.status {
                serverSocket.redialCount = 0
            }
            // TODO refine the lost connection signalling protocol
            if 100 == serverSocket.redialCount {
                // connection has been down a while, notify user
                playMorseElements("........", toneControl)
            }
        }

        // ==================================
        // check the Morse code key input pin
        // ==================================
        keyValue = key.keyPin.Read()
        if( keyValue != lastKeyValue) {
            lastKeyValue = keyValue

            if( rpio.Low == keyValue) {
                toneControl <- rpio.High    // server supresses echo, use side tone instead
                keyToken = "1"
            } else {
                toneControl <- rpio.Low     // server supresses echo, use side tone instead
                keyToken = "0"
            }
            timestamp := strconv.FormatInt(microseconds(), 10)
            // fmt.Println(keyValue, timestamp, "v2 - keyValue: timestamp: version")
            msg := keyToken + timestamp + "v2"
            serverSocket.sendMsg(msg)
        }
        time.Sleep(10 * time.Millisecond)
        if 0 == loopCount % mainLoop30Sec {
            serverSocket.sendMsg("ping")
        }
        loopCount++
    }


} // end func main()

/* end of file */

