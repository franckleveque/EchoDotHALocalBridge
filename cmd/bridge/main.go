package main

import (
	"context"
	"fmt"
	"hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/input/ssdp"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/adapters/output/persistence"
	"hue-bridge-emulator/internal/domain/service"
	"log"
	"net"
	"os"
)

func main() {
	ip := os.Getenv("LOCAL_IP")
	if ip == "" {
		ip = getLocalIP()
	}
	if ip == "" {
		log.Fatal("Could not determine local IP. Set LOCAL_IP environment variable.")
	}

	fmt.Printf("Starting Hue Bridge Emulator on %s\n", ip)

	// Persistance
	configRepo := persistence.NewJSONConfigRepository("/data/config.json")
	if os.Getenv("CONFIG_PATH") != "" {
		configRepo = persistence.NewJSONConfigRepository(os.Getenv("CONFIG_PATH"))
	}

	// HA Client
	haClient := homeassistant.NewClient()

	// Load initial config if exists
	cfg, err := configRepo.Get(context.Background())
	if err != nil {
		log.Printf("Error loading config: %v", err)
	}
	if cfg.HassURL != "" && cfg.HassToken != "" {
		haClient.Configure(cfg.HassURL, cfg.HassToken)
		log.Printf("Home Assistant configured from persisted storage")
	} else {
		// Try env vars for initial config
		hassURL := os.Getenv("HASS_URL")
		hassToken := os.Getenv("HASS_TOKEN")
		if hassURL != "" && hassToken != "" {
			haClient.Configure(hassURL, hassToken)
			cfg.HassURL = hassURL
			cfg.HassToken = hassToken
			configRepo.Save(context.Background(), cfg)
			log.Printf("Home Assistant configured from environment variables")
		} else {
			log.Printf("Home Assistant not configured. Please use the Web Admin interface.")
		}
	}

	bridgeService := service.NewBridgeService(haClient, configRepo)
	bridgeService.Start(context.Background())

	// Start SSDP Server
	ssdpServer := ssdp.NewServer(ip)
	go func() {
		if err := ssdpServer.Start(); err != nil {
			log.Printf("SSDP Server error: %v", err)
		}
	}()

	// Start HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	httpServer := http.NewServer(bridgeService, ip)
	log.Printf("HTTP Server listening on :%s", port)
	if err := httpServer.ListenAndServe(":"+port); err != nil {
		log.Fatal(err)
	}
}

func getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}

			return ip.String()
		}
	}
	return ""
}
