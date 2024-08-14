// Side Channel for communicating metadata about tests to server.
package network

import (
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "strconv"
    "strings"
    "time"

    "wehe-cmdline-client/internal/testdata"
)

const (
    sideChannelPort = 55556
)

type opcode byte // request type to the server

const (
    invalid opcode = 255
    oldDeclareID opcode = 0x30
    receiveID opcode = iota
    ask4permission
    mobileStats
    throughputs
    declareReplay
    analyzeTest
)

type responseCode byte // code representing the status of a response back from the server

const (
    okResponse responseCode = iota
    errorResponse
)

type SideChannel struct {
    id int // ID of SideChannel instance
    conn net.Conn // connection to server
}

// Creates a new SideChannel struct.
// id: ID of the SideChannel instance
// ip: IP of the server to connect to
// Returns new SideChannel struct or any errors
func NewSideChannel(id int, ip string) (SideChannel, error) {
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, sideChannelPort))
    if err != nil {
        return SideChannel{}, nil
    }
    return SideChannel{
        id: id,
        conn: conn,
    }, nil
}

// Tells the server that client wants to run a replay and sends info needed to run the replay.
// userID: unique identifier for the client
// replayID: indicates whether replay is the original or random replay
// replayName: name of the replay
// numMLabTries: number of tries it took to connect to MLab server; 0 if server is not MLab
// testID: ID of the test for given user; i.e. testID can be used to tell how many tests a user has run
// isLastReplay: true if this is the last replay of the test; false otherwise
// publicIP: IP of the client on the test port
// clientVersion: client version of Wehe
// Returns any errors
func (sideChannel SideChannel) SendID(userID string, replayID int, replayName string, numMLabTries int, testID int, isLastReplay bool, publicIP string, clientVersion string) error {
    replayIDString := strconv.Itoa(replayID)
    numMLabTriesString := strconv.Itoa(numMLabTries)
    testIDString := strconv.Itoa(testID)
    isLastReplayString := strings.Title(strconv.FormatBool(isLastReplay))

    message := strings.Join([]string{userID, replayIDString, replayName, numMLabTriesString, testIDString, isLastReplayString, publicIP, clientVersion}, ";")
    fmt.Println(message)
    opcodeAndMessageLength := make([]byte, 4)
    binary.BigEndian.PutUint32(opcodeAndMessageLength, uint32(len(message)))
    opcodeAndMessageLength[0] = byte(receiveID)
    _, err := sideChannel.conn.Write(opcodeAndMessageLength)
    if err != nil {
        return err
    }
    _, err = sideChannel.conn.Write([]byte(message))
    if err != nil {
        return err
    }
    return nil
}

// Asks server if client can run replay.
// Returns a slice containing a status code and information; if status is success, then number of
//     samples per replay is returned as the info; if status is failure, then failure code is
//     returned as the info; can also return errors
func (sideChannel SideChannel) Ask4Permission() ([]string, error) {
    resp, err := sideChannel.sendAndReceive(ask4permission, "")
    if err != nil {
        return nil, err
    }
    permission := strings.Split(resp, ";")
    if len(permission) < 2 {
        return nil, fmt.Errorf("Received improperly formatted permission: %s\n", resp)
    }
    return permission, nil
}

// Send replay duration, throughputs, and sample times to the server after a replay has run.
// replayDuration: the actual amount of time that was used to run the replay
// throughputData: the data rate (in Mbps) of each sample
// sampleTimes: the number of seconds since the replay started that each sample was taken
// Returns a status code from the server indicating success or failure, or an error if the data
//     failed to be sent to the server
func (sideChannel SideChannel) SendThroughputs(replayDuration time.Duration, throughputData []float64, sampleTimes []float64) (string, error) {
    data := [][]float64{throughputData, sampleTimes}
    jsonData, err := json.Marshal(data)
    if err != nil {
        return "", err
    }
    // send in the format <replayDuration>;[[<throughputs>],[sampleTimes]]
    buffer := strconv.FormatFloat(replayDuration.Seconds(), 'f', -1, 64) + ";" + string(jsonData)
    resp, err := sideChannel.sendAndReceive(throughputs, buffer)
    if err != nil {
        return "", err
    }
    return resp, nil
}

// Send a request to run an additional replay in a test. The request to run the first replay in a
// test is included in SendID.
// replayID: the type of replay to run
// replayName: the name of the replay
// isLastReplay: true if this is the last replay in the test; false otherwise
// Returns a slice containing a status code and information; if status is success, then number of
//     samples per replay is returned as the info; if status is failure, then failure code is
//     returned as the info; can also return errors
func (sideChannel SideChannel) DeclareReplay(replayID int, replayName string, isLastReplay bool) ([]string, error) {
    message := strings.Join([]string{strconv.Itoa(replayID), replayName, strings.Title(strconv.FormatBool(isLastReplay))}, ";")
    resp, err := sideChannel.sendAndReceive(declareReplay, message)
    if err != nil {
        return nil, err
    }
    permission := strings.Split(resp, ";")
    if len(permission) < 2 {
        return nil, fmt.Errorf("Received improperly formatted permission: %s\n", resp)
    }
    return permission, nil
}

// Sends a request to analyze the test.
// TODO: finish - rename function and get results back
// Returns the analysis result, or any errors
func (sideChannel SideChannel) AnalyzeTest() (testdata.KS2Result, error) {
    message, err := sideChannel.sendAndReceive(analyzeTest, "")
    if err != nil {
        return testdata.KS2Result{}, err
    }

    var ks2Result testdata.KS2Result
    err = json.Unmarshal([]byte(message), &ks2Result)
    if err != nil {
        return testdata.KS2Result{}, err
    }
    return ks2Result, nil
}

func (sideChannel SideChannel) CleanUp() {
    if sideChannel.conn != nil {
        sideChannel.conn.Close()
    }
}

// Send and receive bytes to the side channel server.
// opcode: the operation number
// message: the data to send to the server
// Returns the server response or any errors
func (sideChannel SideChannel) sendAndReceive(op opcode, message string) (string, error) {
    fmt.Println("sending:", message)

    // send length of request
    // first byte is opcode, last 3 bytes is 24-bit big-endian unsigned message length
    opcodeAndDataLength := make([]byte, 4)
    binary.BigEndian.PutUint32(opcodeAndDataLength, uint32(len(message)))
    opcodeAndDataLength[0] = byte(op)
    _, err := sideChannel.conn.Write(opcodeAndDataLength)
    if err != nil {
        return "", err
    }

    // send request
    _, err = sideChannel.conn.Write([]byte(message))
    if err != nil {
        return "", err
    }

    // read length of response
    messageLengthBytes := make([]byte, 4)
    _, err = io.ReadFull(sideChannel.conn, messageLengthBytes)
    if err != nil {
        return "", err
    }
    messageLength := binary.BigEndian.Uint32(messageLengthBytes)

    // read response
    resp := make([]byte, messageLength)
    _, err = io.ReadFull(sideChannel.conn, resp)
    if err != nil {
        return "", err
    }

    responseCode := resp[0]
    fmt.Println("response code:", responseCode)
    if responseCode == byte(okResponse) {
        fmt.Println("receiving:", string(resp[1:]))
        return string(resp[1:]), nil
    } else if responseCode == byte(errorResponse) {
        return "", fmt.Errorf("Server unable to process request.")
    } else {
        return "", fmt.Errorf("Unknown error.")
    }
    fmt.Println("receiving:", string(resp))
    return string(resp), nil
}
