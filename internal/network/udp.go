// A UDP client to run UDP replays.
package network

import (
    "context"
    "fmt"
    "net"
    "strconv"
    "time"

    "wehe-cmdline-client/internal/testdata"
)

const (
    udpReplayTimeout = 45 * time.Second // each UDP replay is limited to 45 seconds so that user doesn't have to wait forever
)

type UDPClient struct {
    IP string // IP that the client should connect to
    Port int // port that the client should connect to
    Conn *net.UDPConn // the UDP connection to the server
}

// Makes a new UDP client.
// ip: IP of the server
// port: port of the server
// Returns a new UDP client or any errors
func NewUDPClient(ip string, port int) (UDPClient, error) {
    portStr := strconv.Itoa(port)
    udpServer, err := net.ResolveUDPAddr("udp", ip + ":" + portStr)
    if err != nil {
        return UDPClient{}, err
    }
    conn, err := net.DialUDP("udp", nil, udpServer)
    if err != nil {
        return UDPClient{}, err
    }
    return UDPClient{
        IP: ip,
        Port: port,
        Conn: conn,
    }, nil
}

// Sends UDP packets to the server.
// packets: the packets to send to the server
// timing: true if packets should be sent at their timestamps; false otherwise
// ctx: context to help with stopping all UDP sending and receiving threads when error occurs
// cancel: the cancel function to call when error occurs to stop all UDP sending and receiving threads
// errChan: channel to return any errors
func (udpClient UDPClient) SendPackets(packets []testdata.Packet, timing bool, ctx context.Context, cancel context.CancelFunc, errChan chan<- error) {
    startTime := time.Now()
    packetLen := len(packets)
    for i, p := range packets {
        select {
        case <-ctx.Done():
            // another SendPackets or RecvPackets thread has errored out or finished sending packets
            errChan <- nil
            return
        default:
            packet := p.(*testdata.UDPPacket)

            // replays stop after a certain amount of time so that user doesn't have to wait too long
            elapsedTime := time.Now().Sub(startTime)
            if elapsedTime > udpReplayTimeout {
                fmt.Println("TIMEOUT:", elapsedTime, udpReplayTimeout)
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
            _, err := udpClient.Conn.Write(packet.Payload)
            if err != nil {
                cancel()
                errChan <- err
                return
            }
        }
    }
    cancel()
    errChan <- nil
}

// Receives UDP packets from the server.
// ctx: context to help with stopping all UDP sending and receiving threads when error occurs
// cancel: the cancel function to call when error occurs to stop all UDP sending and receiving threads
// errChan: channel to return any errors
func (udpClient UDPClient) RecvPackets(ctx context.Context, cancel context.CancelFunc, errChan chan<- error) {
    for {
        select {
        case <-ctx.Done():
            // another SendPackets or RecvPackets thread has errored out or finished sending packets
            errChan <- nil
            return
        default:
            // don't block trying to read, so that check above can be done to see if another thread has finished
            err := udpClient.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
            if err != nil {
                cancel()
                errChan <- err
                return
            }

            buffer := make([]byte, 4096)
            numBytes, err := udpClient.Conn.Read(buffer)
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    // read timeout to not block reached
                    break
                } else {
                    cancel()
                    errChan <- err
                    return
                }
            }
            fmt.Printf("Received %d bytes from server.\n", numBytes)
        }
    }
}

func (udpClient UDPClient) CleanUp() {
    udpClient.Conn.Close()
}
