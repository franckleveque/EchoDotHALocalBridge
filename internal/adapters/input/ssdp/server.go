package ssdp

import (
	"fmt"
	"log/slog"
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
			slog.Warn("SSDP: skipping interface", "interface", iface.Name, "error", err)
			continue
		}
		slog.Info("SSDP: listening on interface", "interface", iface.Name)
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
			slog.Error("SSDP: read error", "error", err)
			continue
		}

		msg := string(buf[:n])
		slog.Debug("SSDP: received packet", "bytes", n, "from", src, "message", msg)
		if strings.Contains(msg, "M-SEARCH") {
			// Echo Dot 3 often searches for urn:schemas-upnp-org:device:basic:1 or upnp:rootdevice
			if strings.Contains(msg, "urn:schemas-upnp-org:device:basic:1") ||
				strings.Contains(msg, "upnp:rootdevice") ||
				strings.Contains(msg, "ssdp:all") {
				slog.Info("SSDP: responding to M-SEARCH", "from", src)
				s.respond(src)
			}
		}
	}
}

func (s *Server) respond(dest *net.UDPAddr) {
	conn, err := net.DialUDP("udp4", nil, dest)
	if err != nil {
		slog.Error("SSDP: failed to respond", "dest", dest, "error", err)
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

	slog.Info("SSDP: sent response", "dest", dest)
	conn.Write([]byte(resp))
}
