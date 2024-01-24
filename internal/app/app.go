// Provides the main logic for Wehe.
package app

import (
    "bufio"
    "fmt"
    "math/rand"
    "os"
    "strconv"
    "strings"
    "time"
    "unicode"

    "wehe-cmdline-client/internal/config"
    "wehe-cmdline-client/internal/replay"
    "wehe-cmdline-client/internal/server"
)

const (
    charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    userIDLength = 10
    cmdlineUserIDFirstChar = "@"
)

// Run the Wehe app.
// cfg: the configurations to run Wehe with
// Returns any errors
func Run(cfg config.Config) error {
    //history count
    userID, testID := readUserConfig(cfg.UserConfigFile)
    fmt.Println(userID, testID)

    tests, err := replay.ParseTestJSON(cfg.TestsConfigFile, cfg.TestNames, cfg.TestsDir)
    if err != nil {
        return err
    }
    _ = tests

    //set up servers / certs
    var servers []*server.Server
    useMLab, err := server.UseMLab(cfg.ServerDisplay)
    if err != nil {
        return err
    }
    if useMLab {
        // We currently use MLab if the hostname is wehe4.meddle.mobi or if the client is using
        // IPv6. Steps to connect:
        // 1) GET request to MLab site to get JSON of MLab servers that can be connected to.
        // 2) Get the hostname (machine) and websocket authentication URL
        //    (wss://:4443/v0/envelope/access) of a server. The URL is valid for two minutes.
        // 3) Connect to the websocket URL and have connection open for duration of test. The
        //    websocket connection is valid for two minutes.
        // 4) Connect to the SideChannel using the hostname returned by the GET request.
        mlabServers, err := server.GetMLabServers()
        if err != nil {
            return err
        }
        var mlabErrors []string
        for _, mlabServer := range mlabServers {
            if len(servers) == cfg.NumServers {
                // we have the desired number of servers
                break
            }

            srv, err := server.New(mlabServer.Hostname)
            if err != nil {
                mlabErrors = append(mlabErrors, fmt.Sprintf("Error initializing server to %s: %v", mlabServer.Hostname, err))
                continue
            }
            err = srv.OpenWebsocket(mlabServer.AccessToken)
            if err != nil {
                mlabErrors = append(mlabErrors, fmt.Sprintf("Error connecting to %s websocket: %v", mlabServer.Hostname, err))
                continue
            }
            servers = append(servers, srv)
        }
        // In the app, if MLab fails to connect, we fall back to the EC2 server. However, because
        // the command line client is mainly used to test connectivity to MLab, we return an error
        // instead.
        if len(servers) != cfg.NumServers {
            return fmt.Errorf("Initialized only %d/%d MLab servers. Errors:\n%s\n", len(servers), cfg.NumServers, strings.Join(mlabErrors, "\n"))
        }
    } else {
        if cfg.NumServers > 1 {
            return fmt.Errorf("Must connect to MLab (%s) to run more than one concurrent test. Currently connected to %s.\n", server.UseMLabHostname, cfg.ServerDisplay)
        }
        srv, err := server.New(cfg.ServerDisplay)
        if err != nil {
            return err
        }
        servers = append(servers, srv)
    }
    //gen certs, maybe can do it outside of loop

    //flip coin

    //run replays

    //get results

    cleanUp(servers)
    return nil
}

func cleanUp(servers []*server.Server) {
    for _, server := range servers {
        server.MLabWebsocket.Close()
    }
}


//TODO: if above gets too long, move below to app.utils file


// Retrieves the user ID and test ID from a file. Creates new ones if file cannot be opened or
// config is invalid.
// userConfigFile: path of the file containing user ID and test ID
// Returns the user ID and test ID
func readUserConfig(userConfigFile string) (string, int) {
    file, err := os.Open(userConfigFile)
    if err != nil {
        return generateUserID(userIDLength), 0
    }
    fileScanner := bufio.NewScanner(file)
    fileScanner.Split(bufio.ScanLines)

    // first line of file is user ID
    fileScanner.Scan()
    userID := fileScanner.Text()
    // second line of file is test ID
    fileScanner.Scan()
    testID, err := strconv.Atoi(fileScanner.Text())
    if err != nil || !isUserIDValid(userID) || !isTestIDValid(testID) {
        userID = generateUserID(userIDLength)
        testID = 0
    }

    file.Close()

    return userID, testID
}

// Checks if a user ID is valid. A user ID is valid if it contains ten alphanumeric characters.
// However, unlike the iOS and Android apps, the first character is a '@' for the command line
// client.
// userID: the user ID to validate
// Returns true if user ID is valid; false otherwise
func isUserIDValid(userID string) bool {
    if len(userID) == userIDLength && string(userID[0]) == cmdlineUserIDFirstChar {
        for i := 1; i < userIDLength; i++ {
            if !unicode.IsLetter(rune(userID[i])) && !unicode.IsNumber(rune(userID[i])) {
                return false
            }
        }
        return true
    }
    return false
}

// Checks if a test ID is valid. A test ID is valid if it is a non-negative integer.
// testID: the test ID to validate
// Returns true if test ID is valid; false otherwise
func isTestIDValid(testID int) bool {
    return testID > -1
}

// Generates a user ID of random alphanumeric characters, with the first character as a '@'.
// length: total length of the user ID, including the '@'.
// Returns the user ID
func generateUserID(length int) string {
	rand.Seed(time.Now().UnixNano())

	result := make([]byte, length - 1)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return cmdlineUserIDFirstChar + string(result)
}
