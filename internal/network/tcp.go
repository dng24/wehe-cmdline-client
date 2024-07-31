// A TCP client to run TCP replays.
//TODO: this file can probably be combined with the UDP file
package network

import (
    "context"
    "fmt"
    "io"
    "net"
    "time"

    "wehe-cmdline-client/internal/analyzer"
    "wehe-cmdline-client/internal/testdata"
)

const (
    TCPReplayTimeout = 45 * time.Second // each TCP replay is limited to 40 seconds so that user doesn't have to wait forever
    PortReplayTimeout = 30 * time.Second // port replays run only for 30 seconds though
)

type TCPClient struct {
    IP string // IP that the client should connect to
    Port int // port that the client should connect to
    Conn *net.Conn // the TCP connection to the server
    Timeout time.Duration // maximum time to run replay
}

// Makes a new TCP client.
// ip: IP of the server
// port: port of the server
// isPortTest: true if replay is a port test; false otherwise
// Returns a new TCP client or any errors
func NewTCPClient(ip string, port int, isPortTest bool) (TCPClient, error) {
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
    if err != nil {
        return TCPClient{}, err
    }
    timeout := TCPReplayTimeout
    if isPortTest {
        timeout = PortReplayTimeout
    }
    return TCPClient{
        IP: ip,
        Port: port,
        Conn: &conn,
        Timeout: timeout,
    }, nil
}

// Sends TCP packets to the server.
// packets: the packets to send to the server
// timing: true if packets should be sent at their timestamps; false otherwise
// ctx: context to help with stopping all TCP sending and receiving threads when error occurs
// cancel: the cancel function to call when error occurs to stop all TCP sending and receiving threads
// errChan: channel to return any errors
func (tcpClient TCPClient) SendPackets(packets []testdata.Packet, timing bool, ctx context.Context, cancel context.CancelFunc, errChan chan<- error) {
    startTime := time.Now()
    packetLen := len(packets)
    for i, p := range packets {
        select {
        case <-ctx.Done():
            // another SendPackets or RecvPackets thread has errored out
            errChan <- nil
            return
        default:
            packet := p.(*testdata.TCPPacket)

            // replays stop after a certain amount of time so that user doesn't have to wait too long
            elapsedTime := time.Now().Sub(startTime)
            if elapsedTime > tcpClient.Timeout {
                fmt.Println("TIMEOUT:", elapsedTime, tcpClient.Timeout)
                cancel()
                errChan <- nil
                return
            }

            // allows packets to be sent at the time of the timestamp
            if timing {
                sleepTime := startTime.Add(packet.Timestamp).Sub(time.Now())
                time.Sleep(sleepTime)
            }

            fmt.Printf("Sending packet %d/%d at %s\n", i + 1, packetLen, packet.Timestamp)
            _, err := (*tcpClient.Conn).Write(packet.Payload)
            if err != nil {
                cancel()
                errChan <- nil
                return
            }
        }
    }
    errChan <- nil
}

// Receives TCP packets from the server.
// throughputCalculator: analyzer to calculate throughputs
// ctx: context to help with stopping all TCP sending and receiving threads when error occurs
// cancel: the cancel function to call when error occurs to stop all TCP sending and receiving threads
// errChan: channel to return any errors
func (tcpClient TCPClient) RecvPackets(throughputCalculator *analyzer.Analyzer, ctx context.Context, cancel context.CancelFunc, errChan chan<- error) {
    throughputCalculator.Run()
    defer throughputCalculator.Stop()

    for {
        select {
        case <-ctx.Done():
            // another SendPackets or RecvPackets thread has errored out
            errChan <- nil
            return
        default:
            // don't block trying to read, so that check above can be done to see if another thread has finished
            err := (*tcpClient.Conn).SetReadDeadline(time.Now().Add(1 * time.Second))
            if err != nil {
                cancel()
                errChan <- err
                return
            }

            buffer := make([]byte, 4096)
            numBytes, err := (*tcpClient.Conn).Read(buffer)
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    // read timeout to not block reached
                    break
                } else if err == io.EOF {
                    // server finished sending packets and closed its connection
                    errChan <- nil
                    return
                } else {
                    cancel()
                    errChan <- err
                    return
                }
            }

            throughputCalculator.AddBytesRead(numBytes)
            fmt.Printf("Received %d bytes from server.\n", numBytes)
        }
    }
    errChan <- nil
}


func(tcpClient TCPClient) CleanUp() {
    if *tcpClient.Conn != nil {
        (*tcpClient.Conn).Close()
    }
}
