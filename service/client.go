package service

import (
	"net"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/uebian/udp2tcp/config"
)

type Client struct {
	wspool  *TCPConnectionPool
	cfg     *config.Config
	closing bool
}

func NewClient(cfg *config.Config) *Client {
	client := &Client{
		wspool:  &TCPConnectionPool{Collector: Newudp2tcpCollector(), config: cfg},
		closing: false,
		cfg:     cfg,
	}

	return client
}

func (c *Client) Init() error {
	udpListenAddr, err := net.ResolveUDPAddr("udp", c.cfg.UDPListenAddr)
	if err != nil {
		return err
	}

	udpTargetAddr, err := net.ResolveUDPAddr("udp", c.cfg.UDPTargetAddr)
	if err != nil {
		return err
	}

	err = c.wspool.Init(udpListenAddr, udpTargetAddr)
	if err != nil {
		return err
	}
	prometheus.MustRegister(c.wspool.Collector)

	for i := range c.cfg.NMux {
		err = c.wspool.NewConnection(&TCPConnection{TCPURL: c.cfg.TCPURL, ID: i})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Close() error {
	c.closing = true
	c.wspool.Close()
	return nil
}

func (c *Client) ListenAndServe() {
	go c.wspool.udpToWebSocket()
	c.wspool.webSocketToUDP()
}
