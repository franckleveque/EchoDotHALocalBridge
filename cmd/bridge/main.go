package main

import (
	"context"
	"hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/input/ssdp"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/adapters/output/logging"
	"hue-bridge-emulator/internal/adapters/output/persistence"
	"hue-bridge-emulator/internal/domain/model"
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
	levelVar.Set(logging.ParseLevel(os.Getenv("LOG_LEVEL")))
	// The ring logs its own events on a stdout-only logger: logging into the
	// ring from the ring would recurse.
	ringLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar}))
	ring := logging.NewLineRing(500, 4096, ringLogger)
	logger := slog.New(logging.NewHandler(os.Stdout, ring, levelVar))

	ip := os.Getenv("LOCAL_IP")
	if ip != "" {
		logger.Info("Using LOCAL_IP from environment", "ip", ip)
	} else {
		preferred := os.Getenv("PREFERRED_NETWORK")
		ip = getLocalIP(logger, preferred)
		if ip != "" {
			logger.Info("Automatically discovered local IP", "ip", ip)
		}
	}

	if ip == "" {
		logger.Error("Could not determine local IP. Set LOCAL_IP environment variable (e.g. LOCAL_IP=192.168.1.10) or PREFERRED_NETWORK (e.g. PREFERRED_NETWORK=192.168.1.0/24).")
		os.Exit(1)
	}

	logger.Info("Starting Hue Bridge Emulator", "ip", ip, "pid", os.Getpid())

	// Persistance
	configRepo := persistence.NewJSONConfigRepository("/data/config.json", logger)
	if os.Getenv("CONFIG_PATH") != "" {
		configRepo = persistence.NewJSONConfigRepository(os.Getenv("CONFIG_PATH"), logger)
	}

	// HA Client
	haClient := homeassistant.NewClient(logger)

	translatorFactory := translator.NewFactory(logger)
	translatorFactory.Register(model.MappingTypeLight, translator.NewLightStrategy(logger))
	translatorFactory.Register(model.MappingTypeCover, translator.NewCoverStrategy(logger))
	translatorFactory.Register(model.MappingTypeClimate, translator.NewClimateStrategy(logger))
	translatorFactory.Register(model.MappingTypeCustom, translator.NewCustomStrategy(logger))

	// Load initial config if exists
	cfg, err := configRepo.Get(context.Background())
	if err != nil {
		logger.Error("Error loading config", "error", err)
	}
	if cfg.HassURL != "" && cfg.HassToken != "" {
		haClient.Configure(cfg.HassURL, cfg.HassToken)
		logger.Info("Home Assistant configured from persisted storage")
	} else {
		logger.Warn("Home Assistant not configured. Please use the Web Admin interface.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		cancel()
	}()

	bridgeService := service.NewBridgeService(haClient, configRepo, translatorFactory, logger)
	bridgeService.SetIgnoredDomains([]string{"zone.", "sun.", "weather."})
	bridgeService.Start(ctx)

	// Start SSDP Server
	ssdpServer := ssdp.NewServer(ip, logger)
	go func() {
		if err := ssdpServer.Start(); err != nil {
			logger.Error("SSDP Server error", "error", err)
		}
	}()

	// Auth
	authRepo := persistence.NewJSONAuthRepository("/data/auth.json", logger)
	if os.Getenv("AUTH_PATH") != "" {
		authRepo = persistence.NewJSONAuthRepository(os.Getenv("AUTH_PATH"), logger)
	}
	authService := service.NewAuthService(authRepo, logger)

	// Start HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	httpServer := http.NewServer(bridgeService, bridgeService, authService, ip, logger)
	httpServer.SetLogSource(ring)
	logger.Info("HTTP Server listening", "address", "0.0.0.0:"+port)
	if err := httpServer.ListenAndServe(":" + port); err != nil {
		logger.Error("HTTP Server error", "error", err)
		os.Exit(1)
	}
}

func getLocalIP(logger *slog.Logger, preferredNet string) string {
	var preferredSubnet *net.IPNet
	if preferredNet != "" {
		_, subnet, err := net.ParseCIDR(preferredNet)
		if err == nil {
			preferredSubnet = subnet
			logger.Info("Searching for IP in preferred network", "network", preferredNet)
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
			ipStr := ipv4FromAddr(addr)
			if ipStr == "" {
				continue
			}
			ip := net.ParseIP(ipStr)

			logger.Info("Found IPv4 address", "ip", ip.String(), "interface", iface.Name)

			// If we have a preferred network, check if this IP belongs to it
			if preferredSubnet != nil && preferredSubnet.Contains(ip) {
				logger.Info("IP matches preferred network", "ip", ip.String(), "network", preferredNet)
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

// ipv4FromAddr returns the non-loopback IPv4 address held by addr, or "".
// net.IPNet prints as "ip/mask" and net.IPAddr as "ip".
func ipv4FromAddr(addr net.Addr) string {
	host, _, _ := strings.Cut(addr.String(), "/")
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() {
		return ""
	}
	ip = ip.To4()
	if ip == nil {
		return ""
	}
	return ip.String()
}
