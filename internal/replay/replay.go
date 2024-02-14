// Runs one replay.
package replay

import (
    "path"
    "time"

    "wehe-cmdline-client/internal/serverhandler"
)

type ReplayType int

const (
    Original ReplayType = iota
    Random
)

type Replay struct {
    test *Test // the test associated with the replay
    replayID ReplayType // indicates whether this is the original or random replay
    testDir string // path to the directory containing the replay files
    servers []*serverhandler.Server // list of servers to run this replay on
    isLastReplay bool // true if this is the last replay to run; false otherwise
    samplesPerReplay int // number of samples taken per replay
}

// Creates a new Replay struct.
// test: the Test struct associated with the replay
// replayID: indicates whether replay is the original or random replay
// testDir: path to the directory that contains all the test files
// servers: the list of servers that the replay should be run on
// isLastReplay: true if this replay will be run last; false otherwise
// Returns a new Replay struct
func NewReplay(test *Test, replayID ReplayType, testDir string, servers []*serverhandler.Server, isLastReplay bool) Replay {
    return Replay{
        test: test,
        replayID: replayID,
        testDir: testDir,
        servers: servers,
        isLastReplay: isLastReplay,
    }
}

// Runs a replay.
// userID: the unique identifier for a user
// clientVersion: client version of Wehe
// Returns any errors
func (r Replay) Run(userID string, clientVersion string) error {
    replayInfo, err := r.getReplayInfo()
    if err != nil {
        return err
    }

    for id, srv := range r.servers {
        srv.ConnectToSideChannel(id)
    }

    for _, srv := range r.servers {
        err = srv.SendID(replayInfo.IsTCP, replayInfo.CSPair.ServerPort, userID, int(r.replayID), replayInfo.ReplayName, r.test.TestID, r.isLastReplay, clientVersion)
        if err != nil {
            return err
        }
    }

    //TODO: client needs this since it sends ask4perm too fast. fix this by having server send back response for SendID that checks if the SendID input is any good
    time.Sleep(time.Second)

    for _, srv := range r.servers {
        samplesPerReplay, err := srv.Ask4Permission()
        if err != nil {
            return err
        }
        r.samplesPerReplay = samplesPerReplay
    }

    return nil
}

// Gets the packets for a given replay. This function also sets the IsTCP field for Test.
// test: the test associated with the packets
// replayType: the type of replay associated with the packets
// testDir: the directory in which the replay files are located in
// Returns a list of packets for the replay
func (r Replay) getReplayInfo() (ReplayInfo, error) {
    var dataFile string
    if r.replayID == Original {
        dataFile = r.test.DataFile
    } else {
        dataFile = r.test.RandomDataFile
    }
    replayInfo, err := ParseReplayJSON(path.Join(r.testDir, dataFile))
    if err != nil {
        return ReplayInfo{}, err
    }
    return replayInfo, nil
}
