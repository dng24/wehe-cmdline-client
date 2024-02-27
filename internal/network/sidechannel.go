// Side Channel for communicating metadata about tests to server.
package network

import (
    "fmt"
    "net"
    "strconv"
    "strings"
)

const (
    sideChannelPort = 55556
)

type opcode byte // request type to the server

const (
    ask4permission opcode = iota
    mobileStats
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
    _, err := sideChannel.conn.Write([]byte(message))
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

func (sideChannel SideChannel) CleanUp() {
    fmt.Println("CLEANING UP side channel")
    sideChannel.conn.Close()
}

// Send and receive bytes to the side channel server.
// opcode: the operation number
// message: the data to send to the server
// Returns the server response or any errors
func (sideChannel SideChannel) sendAndReceive(op opcode, message string) (string, error) {
    buffer := []byte{byte(op)}
    buffer = append(buffer, []byte(message)...)
    fmt.Println("sending:", buffer)
    _, err := sideChannel.conn.Write(buffer)
    if err != nil {
        return "", err
    }

    resp := make([]byte, 1024)
    n, err := sideChannel.conn.Read(resp)
    if err != nil {
        return "", err
    }

    responseCode := resp[0]
    fmt.Println("response code:", responseCode)
    if responseCode == byte(okResponse) {
        fmt.Println("receiving:", string(resp[1:n]))
        return string(resp[1:n]), nil
    } else if responseCode == byte(errorResponse) {
        return "", fmt.Errorf("Server unable to process request.")
    } else {
        return "", fmt.Errorf("Unknown error.")
    }
    fmt.Println("receiving:", string(resp[:n]))
    return string(resp[:n]), nil
}
