package service

import (
	"fmt"
	"net"
	"time"

	//_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/uebian/udp2tcp/config"
)

type Server struct {
	wspool *TCPConnectionPool
	cfg    *config.Config
	connID int
}

func NewServer(cfg *config.Config) *Server {
	svc := &Server{wspool: &TCPConnectionPool{Collector: Newudp2tcpCollector(), config: cfg}, cfg: cfg}
	return svc
}

func (s *Server) Init() error {
	s.connID = 0
	udpListenAddr, err := net.ResolveUDPAddr("udp", s.cfg.UDPListenAddr)
	if err != nil {
		return err
	}

	udpTargetAddr, err := net.ResolveUDPAddr("udp", s.cfg.UDPTargetAddr)
	if err != nil {
		return err
	}
	err = s.wspool.Init(udpListenAddr, udpTargetAddr)
	prometheus.MustRegister(s.wspool.Collector)
	return err
}

func (s *Server) Close() error {
	s.wspool.Close()
	return nil
}

func (s *Server) NextConnectionID() int {
	s.connID++
	return s.connID
}

func (s *Server) ListenAndServe() {
	go s.wspool.udpToWebSocket()
	go s.wspool.webSocketToUDP()

	listener, err := net.Listen("tcp", s.cfg.TCPListenAddr)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		conn.(*net.TCPConn).SetNoDelay(true)
		go s.wspool.NewConnection(&TCPConnection{
			conn:    conn.(*net.TCPConn),
			ID:      s.NextConnectionID(),
			Timeout: time.Duration(s.cfg.Timeout) * time.Millisecond,
		})
	}
}
