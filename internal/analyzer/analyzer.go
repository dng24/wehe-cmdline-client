// captures information about packets received from replay to calculate throughputs
package analyzer

import (
    "time"
)

type Analyzer struct {
    bytesRead int // number of bytes received from server, updated as client receives the packets during replay
    sampleNumber int // number of samples that have been captured so far
    sampleDuration time.Duration // the number of seconds per sample
    sampleTimes []float64 // number of seconds since the analyzer has started when each sample was taken
    samples []int // the number of bytes received for each sample
    throughputs []float64 // the Mbps for each sample
    ticker *time.Ticker // allows a sample to be taken every sampleDuration seconds

    startTime time.Time // time the replay starts
    ReplayElapsedTime time.Duration // length of replay
}

// Creates a new Analyzer object
// replayLength: time it takes to run the replay (taking into account timeouts as well)
// numberOfSamples: the number of samples that should be captured for the replay
// Returns a new Analyzer object
func NewAnalyzer(replayLength time.Duration, numberOfSamples int) *Analyzer {
    return &Analyzer{
        bytesRead: 0,
        sampleNumber: 0,
        sampleDuration: replayLength / time.Duration(numberOfSamples), // in seconds
        sampleTimes: []float64{},
        samples: []int{},
        throughputs: []float64{},
    }
}

// Begins the capturing of samples. Runs a function to capture a sample every sampleDuration seconds.
func (a *Analyzer) Run() {
    a.startTime = time.Now()
    a.ticker = time.NewTicker(a.sampleDuration)
    go func() {
        for {
            <-a.ticker.C
            a.createSample()
        }
    }()
}

// Captures a sample.
func (a *Analyzer) createSample() {
    a.sampleNumber += 1
    sampleTime := float64(a.sampleNumber) * a.sampleDuration.Seconds()
    a.sampleTimes = append(a.sampleTimes, sampleTime)
    a.samples = append(a.samples, a.bytesRead)
    a.bytesRead = 0
}

// Stop capturing samples.
func (a *Analyzer) Stop() {
    a.ReplayElapsedTime = time.Now().Sub(a.startTime)
    a.ticker.Stop()
}

// Adds number of bytes received by the client. This is the input for the analyzer.
func (a *Analyzer) AddBytesRead(bytesRead int) {
    a.bytesRead += bytesRead
}

// Calculates and returns the throughputs and samples.
func (a *Analyzer) GetThroughputsAndSlices() ([]float64, []int) {
    a.throughputs = []float64{}
    for _, sample := range a.samples {
        megabitsRead := float64(sample) / 125000 // convert bytes to megabits
        throughput := megabitsRead / a.sampleDuration.Seconds() // Mbps
        a.throughputs = append(a.throughputs, throughput)
    }
    return a.throughputs, a.samples
}

// Calculates and returns the average throughput for the replay.
func (a *Analyzer) GetAverageThroughput() float64 {
    sum := 0.0
    for _, throughput := range a.throughputs {
        sum += throughput
    }
    return sum / float64(len(a.throughputs))
}
