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

func (sideChannel SideChannel) CleanUp() {
    fmt.Println("CLEANING UP side channel")
    sideChannel.conn.Close()
}
