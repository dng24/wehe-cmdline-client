package server

import (
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"

    "github.com/gorilla/websocket"
)

const (
    UseMLabHostname = "wehe4.meddle.mobi"
    sideChannelPort = 55556
    resultsURL = "https://%s:56566/Results"
    publicIPURL = "http://%s/WHATSMYIPMAN"
    mlabServersURL = "https://locate.measurementlab.net/v2/nearest/wehe/replay"
)

type Server struct {
    HostName string  // hostname of the server
    IP string // ip of the server
    SideChannelPort int // side channel port number
    ResultsURL string // URL to analyze and get results
    PublicIPURL string // URL for client to get its public IP
    MLabWebsocket *websocket.Conn // websocket connection for MLab
}

func New(hostname string) (*Server, error) {
    ips, err := net.LookupHost(hostname) // do DNS lookup
    if err != nil {
        return nil, err
    }
    return &Server{
        HostName: hostname,
        IP: ips[0],
        SideChannelPort: sideChannelPort,
        ResultsURL: fmt.Sprintf(resultsURL, ips[0]),
        PublicIPURL: fmt.Sprintf(publicIPURL, ips[0]),
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
// Returns client's public IP or an error
func GetClientPublicIP(hostname string) (string, error) {
    resp, err := HTTPGet(fmt.Sprintf(publicIPURL, hostname))
    if err != nil {
        return "", err
    }
    return string(resp), nil
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
