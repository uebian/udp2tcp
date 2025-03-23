package service

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uebian/udp2tcp/config"
)

type TCPConnection struct {
	conn       *net.TCPConn
	TCPURL     string // Empty when on server side, disable autorestart
	closing    bool
	redialM    sync.RWMutex
	redialC    *sync.Cond
	resumeC    *sync.Cond
	heartbeat  chan bool
	ID         int
	needRedial bool
}

type TCPConnectionPool struct {
	wsSendChannel  chan *UDPPacket
	udpSendChannel chan *UDPPacket
	udpListenConn  *net.UDPConn
	udpTargetAddr  net.Addr
	closing        bool
	bufferPool     sync.Pool
	RecvUDPPacket  atomic.Uint64
	SentUDPPacket  atomic.Uint64
	RecvWSPacket   atomic.Uint64
	SentWSPacket   atomic.Uint64
	Collector      *udp2tcpCollector
	config         *config.Config
}

func (conn *TCPConnection) CanDial() bool {
	return conn.TCPURL != ""
}

func (conn *TCPConnection) Dial() {
	// the goroutine won't start if the connection is not dialable
	conn.redialM.Lock()
	for !conn.closing {
		if conn.conn != nil {
			conn.conn.Close()
		}
		tConn, err := net.Dial("tcp", conn.TCPURL)
		for err != nil {
			log.Printf("Connection: %d, failed to re-dial, retry in 5s: %v", conn.ID, err)
			time.Sleep(5 * time.Second)
			tConn, err = net.Dial("tcp", conn.TCPURL)
		}
		log.Printf("Connection: %d, connected successfully", conn.ID)
		conn.conn = tConn.(*net.TCPConn)
		conn.conn.SetNoDelay(true)
		conn.needRedial = false
		conn.resumeC.Signal()
		for !conn.needRedial {
			conn.redialC.Wait()
		}
	}
	conn.conn.Close()
	conn.redialM.Unlock()
}

func (c *TCPConnectionPool) Init(udpListenAddr *net.UDPAddr, udpTargetAddr net.Addr) error {
	var err error
	c.udpListenConn, err = net.ListenUDP("udp", udpListenAddr)
	if err != nil {
		return err
	}
	c.bufferPool = sync.Pool{
		New: func() interface{} {
			return &UDPPacket{}
		},
	}

	c.udpTargetAddr = udpTargetAddr

	c.wsSendChannel = make(chan *UDPPacket, c.config.BufferSize)
	c.udpSendChannel = make(chan *UDPPacket, c.config.BufferSize)

	return nil
}

func (c *TCPConnectionPool) NewConnection(conn *TCPConnection) error {
	conn.redialC = sync.NewCond(&conn.redialM)
	conn.resumeC = sync.NewCond(&conn.redialM)
	conn.heartbeat = make(chan bool, 3)

	if conn.conn == nil {
		conn.needRedial = true
		go conn.Dial()
		conn.redialM.Lock()
		conn.redialC.Signal()
		for conn.needRedial {
			conn.resumeC.Wait()
		}
		conn.redialM.Unlock()
	}
	conn.needRedial = false
	go c.handleConnectionSent(conn)
	go c.handleConnectionListen(conn)
	go c.handleConnectionHeartbeat(conn)
	return nil
}

// Return: can resume
func (conn *TCPConnection) wsFail() bool {
	if conn.CanDial() {
		conn.redialM.Lock()
		conn.needRedial = true
		conn.redialC.Signal()
		for conn.needRedial {
			conn.resumeC.Wait()
		}
		conn.redialM.Unlock()
		return true
	} else {
		conn.closing = true
		return false
	}
}

func (c *TCPConnectionPool) handleConnectionHeartbeat(conn *TCPConnection) {
	for !conn.closing {
		time.Sleep(2 * time.Second)
		conn.heartbeat <- true
	}
	close(conn.heartbeat)
}

func (c *TCPConnectionPool) handleConnectionListen(conn *TCPConnection) {
	c.Collector.wsReadCoroutineGauge.Inc()
	defer c.Collector.wsReadCoroutineGauge.Dec()
	for !conn.closing {
		packet := c.bufferPool.Get().(*UDPPacket)
		conn.redialM.RLock()
		err := ReadPacket(conn.conn, packet)
		conn.redialM.RUnlock()
		if err != nil {
			log.Printf("Connection: %d, Failed to read message from TCP Connection: %v", conn.ID, err)
			c.bufferPool.Put(packet)
			if conn.wsFail() {
				continue
			} else {
				return
			}
		}
		if packet.n == 0 {
			c.bufferPool.Put(packet)
			continue
		}
		c.udpSendChannel <- packet
		c.RecvWSPacket.Add(1)
	}
}

func (c *TCPConnectionPool) handleConnectionSent(conn *TCPConnection) {
	c.Collector.wsWriteCoroutineGauge.Inc()
	defer c.Collector.wsWriteCoroutineGauge.Dec()
	for !conn.closing {
		select {

		case packet, ok := <-c.wsSendChannel:
			if !ok {
				break
			}
			if conn.closing {
				c.wsSendChannel <- packet
				return
			}
			conn.redialM.RLock()
			err := SendPacket(conn.conn, packet)
			conn.redialM.RUnlock()
			// log.Printf("[DEBUG %s] Packet sent to Websocket", time.Now().String())
			if err != nil {
				log.Printf("Connection %d, Failed to send message to TCP Connection: %v", conn.ID, err)
				c.wsSendChannel <- packet
				if conn.wsFail() {
					continue
				} else {
					return
				}
			} else {
				c.SentWSPacket.Add(1)
				c.bufferPool.Put(packet)
			}
		case _, ok := <-conn.heartbeat:
			if !ok {
				break
			}
			if conn.closing {
				return
			}
			conn.redialM.RLock()
			err := SendPacket(conn.conn, &UDPPacket{n: 0})
			// log.Printf("Heartbeat was sent to connection %d", conn.ID)
			conn.redialM.RUnlock()
			if err != nil {
				log.Printf("Connection %d, Failed to send message to TCP Connection: %v", conn.ID, err)
				if conn.wsFail() {
					continue
				} else {
					return
				}
			}
		}
	}
}

func (c *TCPConnectionPool) webSocketToUDP() {
	for {
		packet, ok := <-c.udpSendChannel
		if !ok {
			break
		}
		_, err := c.udpListenConn.WriteTo(packet.content[:packet.n], c.udpTargetAddr)
		// log.Printf("[DEBUG %s] Packet sent to UDP", time.Now().String())
		if err != nil {
			log.Printf("Failed to send message to UDP: %v", err)
		}
		c.SentUDPPacket.Add(1)
		c.bufferPool.Put(packet)
	}
}

func (c *TCPConnectionPool) udpToWebSocket() {
	for !c.closing {
		packet := c.bufferPool.Get().(*UDPPacket)
		n, _, err := c.udpListenConn.ReadFromUDP(packet.content[:])
		// log.Printf("[DEBUG %s] Packet received from UDP", time.Now().String())
		packet.n = uint32(n)
		c.RecvUDPPacket.Add(1)
		if err != nil {
			log.Printf("Failed to read message from UDP: %v", err)
			c.bufferPool.Put(packet)
			continue
		}
		c.wsSendChannel <- packet
	}
}

func (c *TCPConnectionPool) Close() {
	c.closing = true
	close(c.wsSendChannel)
	close(c.udpSendChannel)
	c.udpListenConn.Close()
}
