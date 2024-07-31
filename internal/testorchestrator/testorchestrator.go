// Performs the steps needed to run replays by making requests to the server.
package testorchestrator

import (
    "context"
    "fmt"
    "path"
    "time"

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
}

// Creates a new TestOrchestrator struct.
// test: the Test struct associated with the replay
// replayTypes: the list of types of replays to run during the test
// testDir: path to the directory that contains all the test files
// servers: the list of servers that the replay should be run on
// Returns a new Replay struct
func NewTestOrchestrator(test *testdata.Test, replayTypes []ReplayType, testDir string, servers []*serverhandler.Server) *TestOrchestrator {
    return &TestOrchestrator{
        test: test,
        replayTypes: replayTypes,
        replayID: 0,
        testDir: testDir,
        servers: servers,
        isLastReplay: false,
    }
}

// Runs a replay.
// userID: the unique identifier for a user
// clientVersion: client version of Wehe
// Returns any errors
func (r *TestOrchestrator) Run(userID string, clientVersion string) error {
    replayInfo, err := r.getCurrentReplayInfo()
    if err != nil {
        return err
    }

    defer r.cleanUp()
    err = r.connectToSideChannel()
    if err != nil {
        return err
    }

    err = r.sendID(replayInfo, userID, clientVersion)
    if err != nil {
        return err
    }

    err = r.ask4Permission()
    if err != nil {
        return err
    }

    err = r.sendAndReceivePackets(replayInfo)
    if err != nil {
        return err
    }

    err = r.sendThroughputs()
    if err != nil {
        return err
    }

    r.replayID += 1
    r.isLastReplay = true
    replayInfo, err = r.getCurrentReplayInfo()
    if err != nil {
        return err
    }
    err = r.declareReplay(replayInfo)
    if err != nil {
        return err
    }

    err = r.sendAndReceivePackets(replayInfo)
    if err != nil {
        return err
    }

    err = r.sendThroughputs()
    if err != nil {
        return err
    }

    err = r.analyzeTest()
    if err != nil {
        return err
    }

    return nil
}

// Connect the client to the servers to run test.
// Returns any errors
func (r *TestOrchestrator) connectToSideChannel() error {
    for id, srv := range r.servers {
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
func (r *TestOrchestrator) sendID(replayInfo testdata.ReplayInfo, userID string, clientVersion string) error {
    replayType, err := r.getReplayID()
    if err != nil {
        return err
    }
    // let the server know what replay to run
    for _, srv := range r.servers {
        err = srv.SendID(replayInfo.IsTCP, replayInfo.CSPair.ServerPort, userID, int(replayType), replayInfo.ReplayName, r.test.TestID, r.isLastReplay, clientVersion)
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
func (r *TestOrchestrator) ask4Permission() error {
    // ask the server permission to run replay
    for _, srv := range r.servers {
        samplesPerReplay, err := srv.Ask4Permission()
        if err != nil {
            return err
        }
        r.samplesPerReplay = samplesPerReplay
    }
    return nil
}

// Conduct the test by sending and receiving packets.
// replayInfo: information about the replay
// Returns any errors
func (r *TestOrchestrator) sendAndReceivePackets(replayInfo testdata.ReplayInfo) error {
    // send and receive packets
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    var errChans []chan error
    for _, srv := range r.servers {
        errChan := make(chan error)
        go srv.SendAndReceivePackets(replayInfo, r.samplesPerReplay, r.test.Time, ctx, cancel, errChan)
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
func (r *TestOrchestrator) sendThroughputs() error {
    replayType, err := r.getReplayID()
    if err != nil {
        return err
    }
    // send replay duration and samples to server
    for _, srv := range r.servers {
        averageThroughput, err := srv.SendThroughputs()
        if err != nil {
            return err
        }

        fmt.Println("DEBUG avg thruput:", averageThroughput)
        // TODO: currently only test of last server is stored; figure out what to do with tomography
        switch replayType {
        case Original:
            r.test.OriginalThroughput = averageThroughput
        case Random:
            r.test.RandomThroughput = averageThroughput
        default:
            return fmt.Errorf("Cannot set throughput; invalid test type: %v", replayType)
        }
    }
    return nil
}

// Request to run another replay.
// replayInfo: information about the additional replay
// Returns any errors
func (r *TestOrchestrator) declareReplay(replayInfo testdata.ReplayInfo) error {
    replayType, err := r.getReplayID()
    if err != nil {
        return err
    }
    for _, srv := range r.servers {
        samplesPerReplay, err := srv.DeclareReplay(int(replayType), replayInfo.ReplayName, r.isLastReplay)
        if err != nil {
            return err
        }
        r.samplesPerReplay = samplesPerReplay
    }
    return nil
}

func (r *TestOrchestrator) analyzeTest() error {
    for _, srv := range r.servers {
        err := srv.AnalyzeTest()
        if err != nil {
            return err
        }
    }
    return nil
}

func (r *TestOrchestrator) cleanUp() {
    for _, srv := range r.servers {
        srv.CleanUp()
    }
}

// Gets the replay type of the current replay.
// Returns the replay type or any errors
func (r *TestOrchestrator) getReplayID() (ReplayType, error) {
    if r.replayID < 0 || r.replayID >= len(r.replayTypes) {
        return 0, fmt.Errorf("Replay index %d is out of bounds for a test with %d replays.", r.replayID, len(r.replayTypes))
    }
    return r.replayTypes[r.replayID], nil
}

// Gets the packets for the current replay.
// replayID: the replay ID for which the replay type should be retrieved from replayTypes
// Returns a list of packets for the replay
func (r *TestOrchestrator) getCurrentReplayInfo() (testdata.ReplayInfo, error) {
    replayType, err := r.getReplayID()
    if err != nil {
        return testdata.ReplayInfo{}, err
    }
    var dataFile string
    switch replayType {
    case Original:
        dataFile = r.test.DataFile
    case Random:
        dataFile = r.test.RandomDataFile
    default:
        return testdata.ReplayInfo{}, fmt.Errorf("Invalid test type: %v", replayType)
    }

    replayInfo, err := testdata.ParseReplayJSON(path.Join(r.testDir, dataFile))
    if err != nil {
        return testdata.ReplayInfo{}, err
    }
    return replayInfo, nil
}
