package ssdp

import (
	"fmt"
	"log"
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

	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	started := 0
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		iface := iface
		conn, err := net.ListenMulticastUDP("udp4", &iface, addr)
		if err != nil {
			log.Printf("SSDP: skipping interface %s: %v", iface.Name, err)
			continue
		}
		log.Printf("SSDP: listening on interface %s", iface.Name)
		started++
		go s.listen(conn)
	}

	if started == 0 {
		return fmt.Errorf("SSDP: no multicast interface available")
	}

	select {}
}

func (s *Server) listen(conn *net.UDPConn) {
	buf := make([]byte, 1024)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("SSDP: read error: %v", err)
			continue
		}

		msg := string(buf[:n])
		log.Printf("SSDP: received %d bytes from %s", n, src)
		if strings.Contains(msg, "M-SEARCH") {
			// Echo Dot 3 often searches for urn:schemas-upnp-org:device:basic:1 or upnp:rootdevice
			if strings.Contains(msg, "urn:schemas-upnp-org:device:basic:1") ||
				strings.Contains(msg, "upnp:rootdevice") ||
				strings.Contains(msg, "ssdp:all") {
				log.Printf("SSDP: responding to M-SEARCH from %s", src)
				s.respond(src)
			}
		}
	}
}

func (s *Server) respond(dest *net.UDPAddr) {
	conn, err := net.DialUDP("udp4", nil, dest)
	if err != nil {
		log.Printf("SSDP: failed to respond to %s: %v", dest, err)
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

	log.Printf("SSDP: sent response to %s", dest)
	conn.Write([]byte(resp))
}
