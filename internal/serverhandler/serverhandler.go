// Handles the logic for sending requests and receiving responses from the server.
package serverhandler

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"
    "strconv"

    "github.com/gorilla/websocket"

    "wehe-cmdline-client/internal/network"
    "wehe-cmdline-client/internal/testdata"
)

const (
    UseMLabHostname = "wehe4.meddle.mobi" // hostname used to request MLab server
    resultsURL = "https://%s:56566/Results"
    publicIPURL = "http://%s:%d/WHATSMYIPMAN"
    mlabServersURL = "https://locate.measurementlab.net/v2/nearest/wehe/replay" // used to find which MLab server to use

    ask4PermissionOkStatus = "0"
    ask4PermissionErrorStatus = "1"
    ask4PermissionUnknownReplayMsg = "1"
    ask4PermissionIPInUseMsg = "2"
    ask4PermissionLowResourcesMsg = "3"
    ask4PermissionResourceRetrievalFailMsg = "4"
)

type Server struct {
    HostName string  // hostname of the server
    IP string // ip of the server
    SideChannel network.SideChannel // Side Channel connection
    ResultsURL string // URL to analyze and get results
    PublicIPURL string // URL for client to get its public IP
    MLabWebsocket *websocket.Conn // websocket connection for MLab
    NumMLabTries int // number of tries before successful connection to MLab server
}

// Creates a new Server struct.
// hostname: hostname of the server to connect to
// Returns a new Server or any errors
func New(hostname string) (*Server, error) {
    ips, err := net.LookupHost(hostname) // do DNS lookup
    if err != nil {
        return nil, err
    }
    return &Server{
        HostName: hostname,
        IP: ips[0],
        ResultsURL: fmt.Sprintf(resultsURL, ips[0]),
        PublicIPURL: fmt.Sprintf(publicIPURL, ips[0]),
        NumMLabTries: 0,
    }, nil
}

// Opens a websocket connection.
// websocketURL: the websocket URL (ws:// or wss://) to connect to
// Returns any errors
func (srv *Server) OpenWebsocket(websocketURL string) error {
    ws, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
    if err != nil {
        return err
    }
    srv.MLabWebsocket = ws
    return nil
}

//TODO: move below to new mlab file if this file gets too long

// Determines if MLab servers should be used for the tests.
// hostname: the hostname of the server that the user would like to use
// Returns a boolean, true if MLab servers should be used; false otherwise, or an error
func UseMLab(hostname string) (bool, error) {
    useMLab := true
    if hostname != UseMLabHostname {
        addrs, err := net.LookupHost(hostname)
        if err != nil {
            return false, err
        }
        ipType, err := checkIPAddressType(addrs[0])
        if err != nil {
            return false, err
        }
        if ipType == "IPv4" {
            useMLab = false
        }
    }
    return useMLab, nil
}

// Determines if an IP address is IPv4 or IPv6.
// ipAddress: the IP address
// Returns "IPv4" if the address is a v4 address, "IPv6" if the address is a v6 address, or an error
func checkIPAddressType(ipAddress string) (string, error) {
	addr := net.ParseIP(ipAddress)
	if addr == nil {
		return "", fmt.Errorf("%s is an invalid IP address.\n", ipAddress)
	}

	if ip4 := addr.To4(); ip4 != nil {
		return "IPv4", nil
	}

	if ip6 := addr.To16(); ip6 != nil {
		return "IPv6", nil
	}

	return "", fmt.Errorf("%s is an unknown IP type", ipAddress)
}

// 1 of 3 structs to unmarshal MLab websocket info
type MLabServerResultsJson struct {
    MLabServersJson []MLabServerJson `json:"results"`
}

// 1 of 3 structs to unmarshal MLab websocket info
type MLabServerJson struct {
    Machine string `json:"machine"`
    MLabServerURLs MLabServerURLs `json:"urls"`
}

// 1 of 3 structs to unmarshal MLab websocket info
type MLabServerURLs struct {
    AccessToken string `json:"wss://:4443/v0/envelope/access"`
}

// The hostname and the websocket access token needed to connect to an MLab server
type MLabServer struct {
    Hostname string // hostname of the MLab server
    AccessToken string // websocket URL and access token to allow client to connect to MLab server
}

// Gets a list of MLab servers that can be used to run tests.
// Returns list of MLab servers or an error
func GetMLabServers() ([]MLabServer, error) {
    resp, err := HTTPGet(mlabServersURL)
    if err != nil {
        return nil, err
    }

    var mlabServerResultsJson MLabServerResultsJson
    err = json.Unmarshal(resp, &mlabServerResultsJson)
    if err != nil {
        return nil, err
    }

    var mlabServers []MLabServer
    for _, server := range mlabServerResultsJson.MLabServersJson {
        mlabServer := MLabServer{
            Hostname: "wehe-" + server.Machine,
            AccessToken: server.MLabServerURLs.AccessToken,
        }
        mlabServers = append(mlabServers, mlabServer)
    }
    return mlabServers, nil
}

// Connects to the side channel of the server.
// id: the ID number to assign the side channel instance
// Returns any errors
func (srv *Server) ConnectToSideChannel(id int) error {
    sideChannel, err := network.NewSideChannel(id, srv.IP)
    if err != nil {
        return err
    }
    srv.SideChannel = sideChannel
    return nil
}

// Tells the server that client wants to run a replay.
// isTCP: true if this replay uses TCP; false if it uses UDP
// replayPort: the server port number that the client will send packets to for this replay
// userID: the unique identifier for this user
// replayID: indicates whether this is the original or random replay
// testID: the ID of the test for this specific user
// isLastReplay: true if this replay is the last one in the test to run; false otherwise
// clientVersion: client version of Wehe
// Returns any errors
func (srv *Server) SendID(isTCP bool, replayPort int, userID string, replayID int, replayName string, testID int, isLastReplay bool, clientVersion string) error {
    publicIP, err := getClientPublicIP(srv.HostName, replayPort, isTCP)
    if err != nil {
        return err
    }

    err = srv.SideChannel.SendID(userID, replayID, replayName, srv.NumMLabTries, testID, isLastReplay, publicIP, clientVersion)
    if err != nil {
        return err
    }

    return nil
}

// HTTP GET.
// url: the URL to GET
// Returns the body or an error
func HTTPGet(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    body, err := io.ReadAll(resp.Body)
    resp.Body.Close()
    if resp.StatusCode > 299 {
        return nil, fmt.Errorf("GET Response failed with status code: %d and error:\n%s\n", resp.StatusCode, body)
    }
    if err != nil {
        return nil, err
    }
    return body, nil
}

// Get the client's public IP.
// hostname: hostname of the server
// port: port number to make public IP request
// isTCP: true if test is TCP; false if test is UDP
// Returns client's public IP or an error
func getClientPublicIP(hostname string, port int, isTCP bool) (string, error) {
    if isTCP {
        resp, err := HTTPGet(fmt.Sprintf(publicIPURL, hostname, port))
        if err != nil {
            return "", err
        }
        return string(resp), nil
    } else {
        udpServer, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", hostname, port))
        if err != nil {
            return "", err
        }

        conn, err := net.DialUDP("udp", nil, udpServer)
        if err != nil {
            return "", err
        }
        defer conn.Close()

        _, err = conn.Write([]byte("WHATSMYIPMAN"))
        if err != nil {
            return "", err
        }

        resp := make([]byte, 256)
        numBytes, err := conn.Read(resp)
        if err != nil {
            return "", err
        }

        return string(resp[:numBytes]), nil
    }
}

// Asks the server if replay can be run.
// Returns the number of samples that should be collected per replay if client can run replay;
//     otherwise, returns an error
func (srv *Server) Ask4Permission() (int, error) {
    permission, err := srv.SideChannel.Ask4Permission()
    if err != nil {
        return -1, err
    }
    var errStr string
    if permission[0] == ask4PermissionOkStatus {
        samplesPerReplay, err := strconv.Atoi(permission[1])
        if err != nil {
            return -1, err
        }
        return samplesPerReplay, nil
    } else if permission[0] == ask4PermissionErrorStatus {
        switch permission[1] {
        case ask4PermissionUnknownReplayMsg:
            errStr = "Replay requested does not exist on server."
        case ask4PermissionIPInUseMsg:
            errStr = "A client with this IP is already connected. Try again later."
        case ask4PermissionLowResourcesMsg:
            errStr = "Server is low on resources Try again later."
        case ask4PermissionResourceRetrievalFailMsg:
            errStr = "Unable to determine server resources. Try again later."
        default:
            errStr = "Unknown server error: " + permission[1]
        }
    } else {
        errStr = "Unknown Ask4Permission status code: " + permission[0]
    }
    return -1, fmt.Errorf(errStr + "\n")
}

// Send and receive packets to and from the server.
// replayInfo: information needed to run a replay
// ctx: context to help with stopping all UDP sending and receiving threads when error occurs
// cancel: the cancel function to call when error occurs to stop all UDP sending and receiving threads
// errChan: channel to return any errors
func (srv *Server) SendAndReceivePackets(replayInfo testdata.ReplayInfo, ctx context.Context, cancel context.CancelFunc, errChan chan<- error) {
    if replayInfo.IsTCP {
        tcpClient, err := network.NewTCPClient(srv.IP, replayInfo.CSPair.ServerPort)
        if err != nil {
            cancel()
            errChan <- err
            return
        }
        defer tcpClient.CleanUp()

        sendErrChan := make(chan error)
        recvErrChan := make(chan error)

        // start sender and receiver to send and receive UDP packets to and from the server
        go tcpClient.SendPackets(replayInfo.Packets, !replayInfo.IsPortTest, ctx, cancel, sendErrChan)
        go tcpClient.RecvPackets(ctx, cancel, recvErrChan)

        // wait for sender and receiver to finish
        err = <-sendErrChan
        if err != nil {
            errChan <- err
            return
        }
        err = <-recvErrChan
        if err != nil {
            errChan <- err
            return
        }
    } else {
        // make UDP Client
        udpClient, err := network.NewUDPClient(srv.IP, replayInfo.CSPair.ServerPort)
        if err != nil {
            cancel()
            errChan <- err
            return
        }
        defer udpClient.CleanUp()

        sendErrChan := make(chan error)
        recvErrChan := make(chan error)

        // start sender and receiver to send and receive UDP packets to and from the server
        go udpClient.SendPackets(replayInfo.Packets, !replayInfo.IsPortTest, ctx, cancel, sendErrChan)
        go udpClient.RecvPackets(ctx, cancel, recvErrChan)

        // wait for sender and receiver to finish
        err = <-sendErrChan
        if err != nil {
            errChan <- err
            return
        }
        err = <-recvErrChan
        if err != nil {
            errChan <- err
            return
        }
    }
    errChan <- nil
}

func (srv *Server) CleanUp() {
    srv.SideChannel.CleanUp()
    var err error
    if srv.MLabWebsocket != nil {
        err = srv.MLabWebsocket.Close()
    }
    if err != nil {
        fmt.Printf("Error while cleaning up server: %s\n", err)
    }
}
