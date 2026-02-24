package main

import (
	"fmt"
	"hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/input/ssdp"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/domain/service"
	"log"
	"net"
	"os"
)

func main() {
	hassURL := os.Getenv("HASS_URL")
	hassToken := os.Getenv("HASS_TOKEN")
	if hassURL == "" || hassToken == "" {
		log.Fatal("HASS_URL and HASS_TOKEN must be set")
	}

	ip := os.Getenv("LOCAL_IP")
	if ip == "" {
		ip = getLocalIP()
	}
	if ip == "" {
		log.Fatal("Could not determine local IP. Set LOCAL_IP environment variable.")
	}

	fmt.Printf("Starting Hue Bridge Emulator on %s\n", ip)

	haClient := homeassistant.NewClient(hassURL, hassToken)
	bridgeService := service.NewBridgeService(haClient)

	// Start SSDP Server
	ssdpServer := ssdp.NewServer(ip)
	go func() {
		if err := ssdpServer.Start(); err != nil {
			log.Printf("SSDP Server error: %v", err)
		}
	}()

	// Start HTTP Server
	httpServer := http.NewServer(bridgeService, ip)
	log.Printf("HTTP Server listening on :80")
	if err := httpServer.ListenAndServe(":80"); err != nil {
		log.Fatal(err)
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
