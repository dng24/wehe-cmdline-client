// Performs the steps needed to run replays by making requests to the server.
package testorchestrator

import (
    "context"
    "fmt"
    "math"
    "path"
    "time"

    "wehe-cmdline-client/internal/config"
    "wehe-cmdline-client/internal/serverhandler"
    "wehe-cmdline-client/internal/testdata"
)

type ReplayType int

const (
    Original ReplayType = iota
    Random
)

type TestOrchestrator struct {
    test *testdata.Test // the test associated with the replay
    replayTypes []ReplayType // list of the types of replays to run in the test
    replayID int // the current replay that is being ran, increments by 1 for each replay ran in the test
    testDir string // path to the directory containing the replay files
    servers []*serverhandler.Server // list of servers to run this replay on
    isLastReplay bool // true if this is the last replay to run; false otherwise
    samplesPerReplay int // number of samples taken per replay
    useDefaultThresholds bool // true if default area threshold and KS 2 p-value threshold are used
    areaTestThreshold float64
    ks2PValThreshold float64
    testResults []TestResult // the results of the test for each server, including whether differentiation was present and analysis results
}

type TestResult struct {
    ServerHostname string // hostname that the test took place on
    Result string // Either "No differentiation", "Inconclusive results", or "Differentiation Detected"
    KS2Result testdata.KS2Result // the stats of the result
    AreaThreshold float64 // the area threshold that was used to determine differentiation
    KS2PValueThreshold float64 // the KS 2 p-value threshold that was used to determine differentiation
}

// Creates a new TestOrchestrator struct.
// test: the Test struct associated with the replay
// replayTypes: the list of types of replays to run during the test
// testDir: path to the directory that contains all the test files
// servers: the list of servers that the replay should be run on
// Returns a new Replay struct
func NewTestOrchestrator(test *testdata.Test, replayTypes []ReplayType, cfg config.Config, servers []*serverhandler.Server) *TestOrchestrator {
    return &TestOrchestrator{
        test: test,
        replayTypes: replayTypes,
        replayID: 0,
        testDir: cfg.ReplaysDir,
        servers: servers,
        isLastReplay: false,
        useDefaultThresholds: cfg.UseDefaultThresholds,
        areaTestThreshold: float64(cfg.AreaThreshold) / 100.0,
        ks2PValThreshold: float64(cfg.KS2PValueThreshold) / 100.0,
        testResults: []TestResult{},
    }
}

// Runs a replay.
// userID: the unique identifier for a user
// clientVersion: client version of Wehe
// Returns any errors
func (to *TestOrchestrator) Run(userID string, clientVersion string) ([]TestResult, error) {
    replayInfo, err := to.getCurrentReplayInfo()
    if err != nil {
        return nil, err
    }

    defer to.cleanUp()
    err = to.connectToSideChannel()
    if err != nil {
        return nil, err
    }

    err = to.sendID(replayInfo, userID, clientVersion)
    if err != nil {
        return nil, err
    }

    err = to.ask4Permission()
    if err != nil {
        return nil, err
    }

    err = to.sendAndReceivePackets(replayInfo)
    if err != nil {
        return nil, err
    }

    err = to.sendThroughputs()
    if err != nil {
        return nil, err
    }

    to.replayID += 1
    to.isLastReplay = true
    replayInfo, err = to.getCurrentReplayInfo()
    if err != nil {
        return nil, err
    }
    err = to.declareReplay(replayInfo)
    if err != nil {
        return nil, err
    }

    err = to.sendAndReceivePackets(replayInfo)
    if err != nil {
        return nil, err
    }

    err = to.sendThroughputs()
    if err != nil {
        return nil, err
    }

    err = to.analyzeTest()
    if err != nil {
        return nil, err
    }

    return to.testResults, nil
}

// Connect the client to the servers to run test.
// Returns any errors
func (to *TestOrchestrator) connectToSideChannel() error {
    for id, srv := range to.servers {
        err := srv.ConnectToSideChannel(id)
        if err != nil {
            return err
        }
    }
    return nil
}

// Let the servers know that client wants to run a test. Send information about the test and the
// first replay.
// replayInfo: Information about the replay to run
// userID: the identifier for the device running the replay
// clientVersion: the version of the Wehe client
// Returns any errors
func (to *TestOrchestrator) sendID(replayInfo testdata.ReplayInfo, userID string, clientVersion string) error {
    replayType, err := to.getReplayID()
    if err != nil {
        return err
    }
    // let the server know what replay to run
    for _, srv := range to.servers {
        err = srv.SendID(replayInfo.IsTCP, replayInfo.CSPair.ServerPort, userID, int(replayType), replayInfo.ReplayName, to.test.TestID, to.isLastReplay, clientVersion)
        if err != nil {
            return err
        }
    }

    //TODO: client needs this since it sends ask4perm too fast. fix this by having server send back response for SendID that checks if the SendID input is any good
    time.Sleep(time.Second)
    return nil
}

// Asks the servers if the client can run the replay.
// Returns any errors
func (to *TestOrchestrator) ask4Permission() error {
    // ask the server permission to run replay
    for _, srv := range to.servers {
        samplesPerReplay, err := srv.Ask4Permission()
        if err != nil {
            return err
        }
        to.samplesPerReplay = samplesPerReplay
    }
    return nil
}

// Conduct the test by sending and receiving packets.
// replayInfo: information about the replay
// Returns any errors
func (to *TestOrchestrator) sendAndReceivePackets(replayInfo testdata.ReplayInfo) error {
    // send and receive packets
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    var errChans []chan error
    for _, srv := range to.servers {
        errChan := make(chan error)
        go srv.SendAndReceivePackets(replayInfo, to.samplesPerReplay, to.test.Time, ctx, cancel, errChan)
        errChans = append(errChans, errChan)
    }

    for _, errChan := range errChans {
        err := <-errChan
        if err != nil {
            return err
        }
    }
    return nil
}

// Send the replay duration and throughput samples collected by the client during the replay to the
// servers.
// Returns any errors
func (to *TestOrchestrator) sendThroughputs() error {
    replayType, err := to.getReplayID()
    if err != nil {
        return err
    }
    // send replay duration and samples to server
    for _, srv := range to.servers {
        averageThroughput, err := srv.SendThroughputs()
        if err != nil {
            return err
        }

        fmt.Println("DEBUG avg thruput:", averageThroughput)
        // TODO: currently only test of last server is stored; figure out what to do with tomography
        switch replayType {
        case Original:
            to.test.OriginalThroughput = averageThroughput
        case Random:
            to.test.RandomThroughput = averageThroughput
        default:
            return fmt.Errorf("Cannot set throughput; invalid test type: %v", replayType)
        }
    }
    return nil
}

// Request to run another replay.
// replayInfo: information about the additional replay
// Returns any errors
func (to *TestOrchestrator) declareReplay(replayInfo testdata.ReplayInfo) error {
    replayType, err := to.getReplayID()
    if err != nil {
        return err
    }
    for _, srv := range to.servers {
        samplesPerReplay, err := srv.DeclareReplay(int(replayType), replayInfo.ReplayName, to.isLastReplay)
        if err != nil {
            return err
        }
        to.samplesPerReplay = samplesPerReplay
    }
    return nil
}

// Makes a request to analyze test.
// Returns any errors
func (to *TestOrchestrator) analyzeTest() error {
    for _, srv := range to.servers {
        ks2Result, err := srv.AnalyzeTest()
        if err != nil {
            return err
        }

        to.determineDifferentiation(srv.HostName, ks2Result)
    }
    return nil
}

// Determines whether differentiation was present in the test.
// hostname: hostname the test ran on
// ks2Result: the results of the 2-sample KS test
func (to *TestOrchestrator) determineDifferentiation(hostname string, ks2Result testdata.KS2Result) {
    //area test threshold default is 50%; ks2 p value test threshold default is 1%
    //if default switch is on and one of the throughputs is over 10 Mbps, change the
    //area threshold to 30%, which increases chance of Wehe finding differentiation.
    //If the throughputs are over 10 Mbps, the difference between the two throughputs
    //would need to be much larger than smaller throughputs for differentiation to be
    //triggered, which may confuse users
    //TODO: might have to relook at thresholds and do some formal research on optimal
    // thresholds. Currently thresholds chosen ad-hoc
    if to.useDefaultThresholds && (ks2Result.OriginalAvgThroughput > 10 || ks2Result.RandomAvgThroughput > 10) {
        to.areaTestThreshold = 0.3
    }

    aboveArea := math.Abs(ks2Result.Area0var) >= to.areaTestThreshold
    belowP := ks2Result.KS2pVal < to.ks2PValThreshold

    status := "No Differentiation"
    if aboveArea {
        if belowP {
            status = "Differentiation Detected"
        } else {
            status = "Results Inconclusive"
        }
    }
    testResult := TestResult{
        ServerHostname: hostname,
        Result: status,
        KS2Result: ks2Result,
        AreaThreshold: to.areaTestThreshold,
        KS2PValueThreshold: to.ks2PValThreshold,
    }
    to.testResults = append(to.testResults, testResult)
}

func (to *TestOrchestrator) cleanUp() {
    for _, srv := range to.servers {
        srv.CleanUp()
    }
}

// Gets the replay type of the current replay.
// Returns the replay type or any errors
func (to *TestOrchestrator) getReplayID() (ReplayType, error) {
    if to.replayID < 0 || to.replayID >= len(to.replayTypes) {
        return 0, fmt.Errorf("Replay index %d is out of bounds for a test with %d replays.", to.replayID, len(to.replayTypes))
    }
    return to.replayTypes[to.replayID], nil
}

// Gets the packets for the current replay.
// replayID: the replay ID for which the replay type should be retrieved from replayTypes
// Returns a list of packets for the replay
func (to *TestOrchestrator) getCurrentReplayInfo() (testdata.ReplayInfo, error) {
    replayType, err := to.getReplayID()
    if err != nil {
        return testdata.ReplayInfo{}, err
    }
    var dataFile string
    switch replayType {
    case Original:
        dataFile = to.test.DataFile
    case Random:
        dataFile = to.test.RandomDataFile
    default:
        return testdata.ReplayInfo{}, fmt.Errorf("Invalid test type: %v", replayType)
    }

    replayInfo, err := testdata.ParseReplayJSON(path.Join(to.testDir, dataFile))
    if err != nil {
        return testdata.ReplayInfo{}, err
    }
    return replayInfo, nil
}
