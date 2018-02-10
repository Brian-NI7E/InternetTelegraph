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
const spkrPinBCM    = 10    // spkrPinNumber       = 19 active high
const spkrPinBCML   = 27    // spkrPinNumberL      = 13 active low

// operation constants
const (
        mainLoopMs      = 10
        mainLoop1Sec    = 1000 / mainLoopMs
        mainLoop5Sec    = mainLoop1Sec * 5
        mainLoop30Sec   = mainLoop1Sec * 30
    )



var (
    // TODO create build script to assign buildVesion
    // go build -ldflags "-X main.buildVersion=<version info> ...
    buildVersion    string
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
    spkrPin     rpio.Pin    // active high
    spkrPinL    rpio.Pin    // active low
    command     rpio.State
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



/**
 * Reads configuration information from a file.
 *
 * The hardcoded file name 'config.json' is open and read to
 * populate the 'Config' data structure.  If reading or decoding
 * the file fails, the following default values are used:
 *      Channel = "lobby"
 *      Server  = "morse.autodidacts.io"
 *      Port    = "8000"
 *
 * @ return Config  structure containing application parameters
 */
func getConfiguration() (Config) {

    // allow for future feature of using an OS environment variable, for not, we hardcode it
    os.Setenv("TELEGRAPH_CONFIG_PATH", "config.json")
    file, _ := os.Open(os.Getenv("TELEGRAPH_CONFIG_PATH"))
    decoder := json.NewDecoder(file)

    // allow for future feature of using alternate input methods
    // TODO remove Gpio from Config - it is no longer used
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



/**
 * Initialized the Rpio library.
 * Assign the rpio hardware pin to the variable keyPin.
 * Configure the keyPin as an input.
 * Enable the pull up on the keyPin.
 *
 * @return  int result code indicating success or failure
 */
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



/**
 * Set the initial values for the tone state data structure.
 *
 * @param   ts  'tone' data structure to use
 * @return  ts  the updated 'tone' data structure
 */
func intitializeToneState(ts tone) tone {
    ts.spkrPin = rpio.Pin(spkrPinBCM)
    ts.spkrPinL = rpio.Pin(spkrPinBCML)
    ts.spkrPin.Output()
    ts.spkrPinL.Output()
    ts.command  = rpio.Low      // turn off the tone

    return ts
}



/**
 * Configure the network socket client library
 *
 * @param   config  configuration settings
 * @return  sc      socket client handle
 */
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



/**
 * Connect to the internet-telegraph server
 *
 * @parent  sc      this function is associated with the
 *                  socketClient structure
 * @param   c       the go communication channel
 * @param   state   type of data in the channel
 */
// TODO remove the channel as it is not being used anymore
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



/**
 * Send a string to the internet-telegraph server
 *
 * If sending fails, update the status to allow the caller
 * to attempt a reconnect if desired.
 *
 * @parent  sc      this function is associated wi the
 *                  socketClient structure
 * @param   msg     string contains data to be sent
 */
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



/**
 * Goroutine to listen for messages from the internet-telegraph server
 *
 * Goroutines are a lightweight thread of execution.  That means that
 * once started, the routine continues to run without needing to be
 * called by the main loop.
 *
 * @parent  sc      this function is associated wi the
 *                  socketClient structure
 * @param   c       the go communication channel
 * @param   state   type of data in the channel
 */
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



/**
 * Goroutine to control whether the Morse code sounder should make a sound.
 *
 * Goroutines are a lightweight thread of execution.  That means that
 * once started, the routine continues to run without needing to be
 * called by the main loop.
 *
 * @parent  t       this function is associated wi the
 *                  tone structure
 * @param   c       the go communication channel
 * @param   state   type of data in the channel
 */
func (t *tone) control(c chan rpio.State) {
    // TODO add timeout to key down messages from server
    // TOOD allow local key down to run forever ???
    var command rpio.State
    for {
        command = <-c
        t.spkrPin.Write(command)
        if command == rpio.High {
            t.spkrPinL.Write(rpio.Low)
        } else {
            t.spkrPinL.Write(rpio.High)
        }
    }
}




/**
 * Goroutine to time the sounding of 'dit', 'dah', and space characters.
 *
 * Goroutines are a lightweight thread of execution.  That means that
 * once started, the routine continues to run without needing to be
 * called by the main loop.
 *
 * @param   c       the go communication channel
 * @param   state   type of data in the channel
 */
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




// TODO repurpose this to encode strings into Morse code.
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



/**
 * Convert the UnixNano, nanosecond timer value to microseconds.
 *
 * @return  int64   value of operating system timer in microseconds
 */
func microseconds() int64 {
    t := time.Now().UnixNano()
    us := t / int64(time.Microsecond)
    return us
}




/**
 * The main function.
 */
func main() {
    /**
     * Inform the user that the program has started
     */
    fmt.Println("internet-telegraph starting - version", buildVersion)


    /**
     * Create local variables.
     */
    var keyValue rpio.State
    var lastKeyValue rpio.State  = rpio.High    // will a pull up un press = High
    var keyToken string = "0"                   // default to no tone
    var loopCount   int = 0


    /**
     * Get the application configuration information
     */
    config  := getConfiguration()


    /**
     * Initialize the hardware.
     */
    initializeRpio()                            // Initialize the Raspberry Pi IO library
    defer rpio.Close()                          // close and cleanup rpio when main closes



    /**
     * Initialize the application elements.
     */
    toneState       =   intitializeToneState(toneState)
    toneControl     :=  make(chan rpio.State)   // create channel to communicate with tone
    go toneState.control( toneControl)          // launch toneState.control Goroutine
    serverSocket    :=  initializeSocketClient(config)
    serverSocket.dial( toneControl)             // establish connection to server

    if SC_CONNECTED == serverSocket.status {
        playMorseElements(".--. --- ... - ..... ----. ----.", toneControl)  // play "POST599"
    }

    go serverSocket.listen(toneControl)         // launch serverSocket.listen Goroutine




    /**
     * Start the main application loop.  This loop runs forever.
     */
    for {
        // TODO organize main loop as a scheduler
        // This section of the main loop runs everytime
        // This section of the main loop runs every 5 seconds
        // This section of the main loop runs every 30 seconds


        /**
         * Verify the connection to the internet-telegraph server
         * Reconnect to the server is required.
         *
         * Attempt several immediate 'silent' reconnects.  If these fail to
         * reconnect the server, attempt reconnets at a slower rate.  Signify
         * a successful reconnect by playing 'dit dit' on the Morse code
         * sounder.  If the server remains disconnected for an extended
         * period of time, inform the user my playing (8) 'dits' on the Morse
         * code sounder.
         */
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


        /**
         * Poll the Morse code key input and determine if its state has changed.
         * if the state has changed, start or stop the tone on the Morse code sounder.
         */
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


        /**
         * Allow the application to sleep.  This allows other computer to perform
         * other tasks, and reduces the amount of energy used.
         */
        time.Sleep(10 * time.Millisecond)
        if 0 == loopCount % mainLoop30Sec {
            serverSocket.sendMsg("ping")
        }


        /**
         * Increment the loop control counter
         */
        loopCount++
    }


} // end func main()

/* end of file */

