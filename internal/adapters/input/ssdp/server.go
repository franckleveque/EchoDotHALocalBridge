package ssdp

import (
	"fmt"
	"net"
	"strings"
)

type Server struct {
	ip   string
	port int
}

func NewServer(ip string) *Server {
	return &Server{ip: ip, port: 80}
}

func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return err
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return err
	}

	buf := make([]byte, 1024)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		msg := string(buf[:n])
		if strings.Contains(msg, "M-SEARCH") {
			// Echo Dot 3 often searches for urn:schemas-upnp-org:device:basic:1 or upnp:rootdevice
			if strings.Contains(msg, "urn:schemas-upnp-org:device:basic:1") ||
			   strings.Contains(msg, "upnp:rootdevice") ||
			   strings.Contains(msg, "ssdp:all") {
				s.respond(src)
			}
		}
	}
}

func (s *Server) respond(dest *net.UDPAddr) {
	conn, err := net.DialUDP("udp4", nil, dest)
	if err != nil {
		return
	}
	defer conn.Close()

	resp := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"CACHE-CONTROL: max-age=100\r\n"+
		"EXT:\r\n"+
		"LOCATION: http://%s:%d/description.xml\r\n"+
		"SERVER: FreeRTOS/6.0.5, UPnP/1.1, IpBridge/1.17.0\r\n"+
		"ST: urn:schemas-upnp-org:device:basic:1\r\n"+
		"USN: uuid:2f402f80-da50-11e1-9b23-001788102201::urn:schemas-upnp-org:device:basic:1\r\n\r\n", s.ip, s.port)

	conn.Write([]byte(resp))
}
