// Provides the main logic for the Wehe command line client.
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
    "wehe-cmdline-client/internal/testorchestrator"
    "wehe-cmdline-client/internal/serverhandler"
    "wehe-cmdline-client/internal/testdata"
)

const (
    charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    userIDLength = 10
    cmdlineUserIDFirstChar = "@"
)

// Run the Wehe command line client.
// cfg: the configurations to run Wehe with
// version: version number of Wehe
// Returns any errors
func Run(cfg config.Config, version string) error {
    //history count
    userID, testID := readUserConfig(cfg.UserConfigFile)
    fmt.Println(userID, testID)

    tests, err := testdata.ParseTestJSON(cfg.TestsConfigFile, cfg.TestNames)
    if err != nil {
        return err
    }

    //set up servers / certs
    var servers []*serverhandler.Server
    useMLab, err := serverhandler.UseMLab(cfg.ServerDisplay)
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
        mlabServers, err := serverhandler.GetMLabServers()
        if err != nil {
            return err
        }
        var mlabErrors []string
        numTries := 0 // number tries before successful connection to an MLab server
        for _, mlabServer := range mlabServers {
            if len(servers) == cfg.NumServers {
                // we have the desired number of servers
                break
            }

            numTries += 1

            srv, err := serverhandler.New(mlabServer.Hostname)
            if err != nil {
                mlabErrors = append(mlabErrors, fmt.Sprintf("Error initializing server to %s: %v", mlabServer.Hostname, err))
                continue
            }
            defer srv.CleanUp()

            err = srv.OpenWebsocket(mlabServer.AccessToken)
            if err != nil {
                mlabErrors = append(mlabErrors, fmt.Sprintf("Error connecting to %s websocket: %v", mlabServer.Hostname, err))
                continue
            }
            srv.NumMLabTries = numTries
            numTries = 0
            servers = append(servers, srv)
        }
        // In the app, if MLab fails to connect, we fall back to the EC2 serverhandler. However, because
        // the command line client is mainly used to test connectivity to MLab, we return an error
        // instead.
        if len(servers) != cfg.NumServers {
            return fmt.Errorf("Initialized only %d/%d MLab servers. Errors:\n%s\n", len(servers), cfg.NumServers, strings.Join(mlabErrors, "\n"))
        }
    } else {
        if cfg.NumServers > 1 {
            return fmt.Errorf("Must connect to MLab (%s) to run more than one concurrent test. Currently connected to %s.\n", serverhandler.UseMLabHostname, cfg.ServerDisplay)
        }
        srv, err := serverhandler.New(cfg.ServerDisplay)
        if err != nil {
            return err
        }
        servers = append(servers, srv)
    }
    //gen certs, maybe can do it outside of loop

    //flip coin
    replayOrder := generateReplayOrder()

    //run replays
    for _, test := range tests {
        testID += 1
        test.TestID = testID
        r := testorchestrator.NewTestOrchestrator(test, replayOrder, cfg, servers)
        testResults, err := r.Run(userID, version)
        if err != nil {
            return err
        }
        for _, result := range testResults {
            fmt.Printf("Test result for %s:\n\tStatus: %s\n\tOriginal Throughput: %f Mbps\n\tRandom Throughput: %f Mbps\n\tServer: %s\n\tArea Threshold: %f\n\tKS2 P-Value Threshold: %f\n",
                test.Name, result.Result, result.KS2Result.OriginalAvgThroughput, result.KS2Result.RandomAvgThroughput, result.ServerHostname, result.AreaThreshold, result.KS2PValueThreshold)
        }
    }
    return nil
}

// Randomly determines whether the original or random replay will be run first.
// Returns the types of replays in the order that they will be run
func generateReplayOrder() []testorchestrator.ReplayType {
    rand.Seed(time.Now().UnixNano())
    if (rand.Intn(2) == 0) {
        return []testorchestrator.ReplayType{testorchestrator.Original, testorchestrator.Random}
    } else {
        return []testorchestrator.ReplayType{testorchestrator.Random, testorchestrator.Original}
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
