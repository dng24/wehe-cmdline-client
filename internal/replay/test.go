// Parses and provides the tests that users would like to run.
package replay

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "strings"
)

// All the information we need to run a test.
type Test struct {
    Name string `json:"name"` // pretty name of the test (name that is displayed to the user in the app)
    Time int `json:"time"` // number seconds needed to run both replays; port time not accurate - we want to run them as fast as possible
    Image string `json:"image"` // filename of test icon (used for entering test names on command line bc this name has no spaces)
    DataFile string `json:"datafile"` // filename of the original replay
    RandomDataFile string `json:"randomdatafile"` // filename of the random replay

    //IsTCP bool // true if test sends TCP packets; false if it sends UDP packets
    OriginalThroughput float64 // average throughput of the original replay
    RandomThroughput float64 // average throughput of random replay
    TestID int // the ID for the replay for this specific user
}

type ReplayInfo struct {
    Packets []Packet
    CSPair CSPair
    ReplayName string
    IsTCP bool
}

// Either a TCPPacket or UDPPacket.
type Packet interface {

}

// A TCP packet to be sent as part of a replay.
type TCPPacket struct {
    CSPair string // the client & server of original packet capture, in the form {client_IP}.{client_port}-{server_IP}.{server_port}
    Timestamp float64 // time since the start of the replay that this packet should be sent
    Payload []byte // the bytes to send to the server
    ResponseLength int // the expected length of response to this packet
    ResponseHash string // the expected hash of the response
}

func newTCPPacket(csPair string, timestamp float64, payload string, responseLength int, responseHash string) (TCPPacket, error) {
    payloadBytes, err := hex.DecodeString(payload)
    if err != nil {
        return TCPPacket{}, err
    }
    return TCPPacket{
        CSPair: csPair,
        Timestamp: timestamp,
        Payload: payloadBytes,
        ResponseLength: responseLength,
        ResponseHash: responseHash,
    }, nil
}

// A UDP packet to be sent as part of a replay.
type UDPPacket struct {
    CSPair string // the client & server of original packet capture, in the form {client_IP}.{client_port}-{server_IP}.{server_port}
    Timestamp float64 // time since the start of the replay that this packet should be sent
    Payload []byte // the bytes to send to the server
    End bool // ???
}

func newUDPPacket(csPair string, timestamp float64, payload string, end bool) (UDPPacket, error) {
    payloadBytes, err := hex.DecodeString(payload)
    if err != nil {
        return UDPPacket{}, err
    }
    return UDPPacket{
        CSPair: csPair,
        Timestamp: timestamp,
        Payload: payloadBytes,
        End: end,
    }, nil
}

// The structure that replay files get unpacked into.
type ReplayFilePacket struct {
    CSPair string `json:"c_s_pair"` // the client & server of original packet capture, in the form {client_IP}.{client_port}-{server_IP}.{server_port}
    Timestamp float64 `json:"timestamp"` // time since the start of the replay that this packet should be sent
    Payload string `json:"payload"` // the bytes to send to the server
    ResponseLength *int `json:"response_len"` // the expected length of response to this packet *TCP only field
    ResponseHash *string `json:"response_hash"` // the expected hash of the response *TCP only field
    End *bool `json:"end"` // ???
}

// Represents a client-server pair. Every replay file recorded has a CSPair.
type CSPair struct {
    ClientIP string // client IP of the replay file
    ClientPort int // client port of the replay file
    ServerIP string // server IP of the replay file
    ServerPort int // server port of the replay file
}

// Converts a string CSPair into a CSPair struct.
// csPair: a CSPair string in the format <client_ip>.<client_port>-<server_ip>.<server_port>
// Returns a CSPair or any errors
func newCSPair(csPair string) (CSPair, error) {
    clientServer := strings.Split(csPair, "-")
    if len(clientServer) != 2 {
        return CSPair{}, fmt.Errorf("CSPair '%s' is invalid. Should be in format <client_ip>.<client_port>-<server_ip>.<server_port>", csPair)
    }

    lastDotIndex := strings.LastIndex(clientServer[0], ".")
	clientIP := clientServer[0][:lastDotIndex]
    clientPort, err := strconv.Atoi(clientServer[0][lastDotIndex + 1:])
	if err != nil {
        return CSPair{}, err
    }

    lastDotIndex = strings.LastIndex(clientServer[1], ".")
    serverIP := clientServer[1][:lastDotIndex]
    serverPort, err := strconv.Atoi(clientServer[1][lastDotIndex + 1:])
    if err != nil {
        return CSPair{}, err
    }

    return CSPair{
        ClientIP: clientIP,
        ClientPort: clientPort,
        ServerIP: serverIP,
        ServerPort: serverPort,
    }, nil
}

// Loads the tests from disk.
// testsConfigFile: the configuration file name containing information about all the tests
// testNames: the names of the tests that the user would like to run. Test names should match
//            Test.Image
// Returns a list of tests or an error
func ParseTestJSON(testsConfigFile string, testNames []string) ([]*Test, error) {
    data, err := os.ReadFile(testsConfigFile)
    if err != nil {
        return nil, err
    }

    var allTests []Test
    err = json.Unmarshal(data, &allTests)
    if err != nil {
        return nil, err
    }

    var userRequestedTests []*Test
    var validTestNames []string
    for _, test := range allTests {
        if (containsString(testNames, test.Image)) {
            tmpTest := test
            userRequestedTests = append(userRequestedTests, &tmpTest)
            validTestNames = append(validTestNames, test.Image)
        }
    }

    // make sure there aren't any invalid test names that user entered
    err = checkValidTestNames(testNames, validTestNames)
    if err != nil {
        return nil, err
    }

    return userRequestedTests, err
}

// TODO: refactor so that this returns a struct that contains []packet, isTCP, and replay name
// Parses a test file.
// replayFile: file path to the test file
// Returns a list of packets to send to the server that make up the test and true if packets are
//     TCP, false if packets are UDP, or an error
func ParseReplayJSON(replayFile string) (ReplayInfo, error) {
    data, err := os.ReadFile(replayFile)
    if err != nil {
        return ReplayInfo{}, err
    }

    var jsonData []json.RawMessage
    err = json.Unmarshal(data, &jsonData)
    if err != nil {
        return ReplayInfo{}, err
    }

    //TODO: can we get rid of udp client ports, tcp csps, and replay name in tests files and just keep the Q - would make json parsing a lot simplier; can get rid of block below
    // fan and i think we can get rid of udp client ports and csps - double check that, and see if replay name needed
    var replayFilePackets []ReplayFilePacket
    if len(jsonData) > 0 {
		err := json.Unmarshal(jsonData[0], &replayFilePackets)
		if err != nil {
			return ReplayInfo{}, err
		}
	}

    var packets []Packet
    var isTCP bool
    if replayFilePackets[0].ResponseLength != nil {
        // make the TCP packets to be sent to the server
        isTCP = true
        for _, replayFilePacket := range replayFilePackets {
            //TODO: see if test files can replace null with "" in response_hash field; if so, this code is not needed
            var hash string
            if replayFilePacket.ResponseHash == nil {
                hash = ""
            } else {
                hash = *replayFilePacket.ResponseHash
            }

            tcpPacket, err := newTCPPacket(replayFilePacket.CSPair, replayFilePacket.Timestamp, replayFilePacket.Payload, *replayFilePacket.ResponseLength, hash)
            if err != nil {
                return ReplayInfo{}, err
            }

            packets = append(packets, &tcpPacket)
        }
    } else {
        // make the UDP packets to be sent to the server
        isTCP = false
        for _, replayFilePacket := range replayFilePackets {
            udpPacket, err := newUDPPacket(replayFilePacket.CSPair, replayFilePacket.Timestamp, replayFilePacket.Payload, *replayFilePacket.End)
            if err != nil {
                return ReplayInfo{}, err
            }

            packets = append(packets, &udpPacket)
        }
    }

    //TODO: also very sketch, fix test file format, as mentioned above
    var csPair CSPair
    if isTCP {
        var csPairs []string
        err := json.Unmarshal(jsonData[2], &csPairs)
        if err != nil {
            return ReplayInfo{}, err
        }
        csPair, err = newCSPair(csPairs[0]) // we currently only have 1 cs pair
        if err != nil {
            return ReplayInfo{}, err
        }
    } else {
        csPair, err = newCSPair(packets[0].(*UDPPacket).CSPair)
        if err != nil {
            return ReplayInfo{}, err
        }
    }
    replayName := strings.Split(string(jsonData[3]), "\"")[1]

    return ReplayInfo{
        Packets: packets,
        CSPair: csPair,
        ReplayName: replayName,
        IsTCP: isTCP,
    }, nil
}

// Checks if slice contains a string.
// slice: slice of strings to search from
// target: the string to look for in the slice
// Returns true if target is in slice; false otherwise
func containsString(slice []string, target string) bool {
    for _, element := range slice {
        if element == target {
            return true
        }
    }
    return false
}

// Determines if there are any test names provided by the user that are not valid.
// testNames: the list of test names that was given by the user
// validTestNames: a valid list of test names
// Returns an error if a user-provided test name is not in the list of valid test names
func checkValidTestNames(testNames []string, validTestNames []string) error {
    // add all valid tests to a map
    validTestNamesMap := make(map[string]bool)
    for _, validTestName := range validTestNames {
        validTestNamesMap[validTestName] = true
        fmt.Println(validTestName)
    }

    // see if each user provided test is in the map
    var invalidTestNames []string
    for _, testName := range testNames {
        if !validTestNamesMap[testName] {
            invalidTestNames = append(invalidTestNames, testName)
        }
    }
    if len(invalidTestNames) == 0 {
        return nil
    } else {
        return fmt.Errorf("The following are invalid test names: %v\n", invalidTestNames)
    }
}
