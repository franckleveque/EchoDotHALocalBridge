package main

import (
	"context"
	"hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/input/ssdp"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/adapters/output/persistence"
	"hue-bridge-emulator/internal/domain/service"
	"hue-bridge-emulator/internal/domain/translator"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	levelVar := &slog.LevelVar{}
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		levelVar.Set(slog.LevelDebug)
	case "WARN":
		levelVar.Set(slog.LevelWarn)
	case "ERROR":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar})
	slog.SetDefault(slog.New(handler))

	ip := os.Getenv("LOCAL_IP")
	if ip != "" {
		slog.Info("Using LOCAL_IP from environment", "ip", ip)
	} else {
		preferred := os.Getenv("PREFERRED_NETWORK")
		ip = getLocalIP(preferred)
		if ip != "" {
			slog.Info("Automatically discovered local IP", "ip", ip)
		}
	}

	if ip == "" {
		slog.Error("Could not determine local IP. Set LOCAL_IP environment variable (e.g. LOCAL_IP=192.168.1.10) or PREFERRED_NETWORK (e.g. PREFERRED_NETWORK=192.168.1.0/24).")
		os.Exit(1)
	}

	slog.Info("Starting Hue Bridge Emulator", "ip", ip, "pid", os.Getpid())

	// Persistance
	configRepo := persistence.NewJSONConfigRepository("/data/config.json")
	if os.Getenv("CONFIG_PATH") != "" {
		configRepo = persistence.NewJSONConfigRepository(os.Getenv("CONFIG_PATH"))
	}

	// HA Client
	haClient := homeassistant.NewClient()

	translatorFactory := translator.NewFactory()
	translatorFactory.Register(model.MappingTypeLight, &translator.LightStrategy{})
	translatorFactory.Register(model.MappingTypeCover, &translator.CoverStrategy{})
	translatorFactory.Register(model.MappingTypeClimate, &translator.ClimateStrategy{})
	translatorFactory.Register(model.MappingTypeCustom, &translator.CustomStrategy{})

	// Load initial config if exists
	cfg, err := configRepo.Get(context.Background())
	if err != nil {
		slog.Error("Error loading config", "error", err)
	}
	if cfg.HassURL != "" && cfg.HassToken != "" {
		haClient.Configure(cfg.HassURL, cfg.HassToken)
		slog.Info("Home Assistant configured from persisted storage")
	} else {
		slog.Warn("Home Assistant not configured. Please use the Web Admin interface.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("Shutting down...")
		cancel()
	}()

	bridgeService := service.NewBridgeService(haClient, configRepo, translatorFactory)
	bridgeService.SetIgnoredDomains([]string{"zone.", "sun.", "weather."})
	bridgeService.Start(ctx)

	// Start SSDP Server
	ssdpServer := ssdp.NewServer(ip)
	go func() {
		if err := ssdpServer.Start(); err != nil {
			slog.Error("SSDP Server error", "error", err)
		}
	}()

	// Auth
	authRepo := persistence.NewJSONAuthRepository("/data/auth.json")
	if os.Getenv("AUTH_PATH") != "" {
		authRepo = persistence.NewJSONAuthRepository(os.Getenv("AUTH_PATH"))
	}
	authService := service.NewAuthService(authRepo)

	// Start HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	httpServer := http.NewServer(bridgeService, bridgeService, authService, ip)
	slog.Info("HTTP Server listening", "address", "0.0.0.0:"+port)
	if err := httpServer.ListenAndServe(":"+port); err != nil {
		slog.Error("HTTP Server error", "error", err)
		os.Exit(1)
	}
}

func getLocalIP(preferredNet string) string {
	var preferredSubnet *net.IPNet
	if preferredNet != "" {
		_, subnet, err := net.ParseCIDR(preferredNet)
		if err == nil {
			preferredSubnet = subnet
			slog.Info("Searching for IP in preferred network", "network", preferredNet)
		}
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var bestIP string
	for _, iface := range interfaces {
		// Skip down and loopback
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

			slog.Info("Found IPv4 address", "ip", ip.String(), "interface", iface.Name)

			// If we have a preferred network, check if this IP belongs to it
			if preferredSubnet != nil && preferredSubnet.Contains(ip) {
				slog.Info("IP matches preferred network", "ip", ip.String(), "network", preferredNet)
				return ip.String()
			}

			// Prioritize physical interfaces (eth, en, wl) over virtual ones (docker, veth, br, utun)
			name := strings.ToLower(iface.Name)
			if strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "en") || strings.HasPrefix(name, "wl") {
				if preferredSubnet == nil {
					return ip.String()
				}
			}

			if bestIP == "" {
				bestIP = ip.String()
			}
		}
	}
	return bestIP
}
