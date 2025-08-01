package ngrok

import (
	"context"
	"fmt"
	"log"
	"os"

	"staccato/internal/config"

	"github.com/joho/godotenv"
	"golang.ngrok.com/ngrok/v2"
)

// Service represents the ngrok tunnel service
type Service struct {
	config *config.NgrokConfig
	agent  ngrok.Agent
	tunnel ngrok.EndpointForwarder
}

// NewService creates a new ngrok service instance
func NewService(cfg *config.NgrokConfig) (*Service, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Load .env file if it exists (for auth token)
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Printf("Warning: Could not load .env file: %v", err)
		}
	}

	// Get auth token from environment variable if not set in config
	authToken := cfg.AuthToken
	if authToken == "" {
		authToken = os.Getenv("NGROK_AUTHTOKEN")
	}

	if authToken == "" {
		return nil, fmt.Errorf("ngrok auth token not found. Set NGROK_AUTHTOKEN in .env file or config")
	}

	// Create ngrok agent
	agent, err := ngrok.NewAgent(ngrok.WithAuthtoken(authToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create ngrok agent: %v", err)
	}

	return &Service{
		config: cfg,
		agent:  agent,
	}, nil
}

// StartTunnel starts the ngrok tunnel
func (s *Service) StartTunnel(ctx context.Context, localAddress string) error {
	if s == nil {
		return nil // Service is disabled
	}

	log.Println("🌐 Starting ngrok tunnel...")

	// Build endpoint options
	var endpointOpts []ngrok.EndpointOption

	// Add domain if specified
	if s.config.Domain != "" {
		endpointOpts = append(endpointOpts, ngrok.WithURL(s.config.Domain))
	}

	// Add OAuth authentication if enabled
	if s.config.EnableAuth {
		trafficPolicy := fmt.Sprintf(`
on_http_request:
  - actions:
      - type: oauth
        config:
          provider: %s
`, s.config.AuthProvider)
		endpointOpts = append(endpointOpts, ngrok.WithTrafficPolicy(trafficPolicy))
	}

	// Create the tunnel
	tunnel, err := s.agent.Forward(ctx, ngrok.WithUpstream(localAddress), endpointOpts...)
	if err != nil {
		return fmt.Errorf("failed to create ngrok tunnel: %v", err)
	}

	s.tunnel = tunnel

	log.Printf("✅ Ngrok tunnel active!")
	log.Printf("🌍 Public URL: %s", tunnel.URL().String())
	log.Printf("🔗 Forwarding to: %s", localAddress)

	if s.config.EnableAuth {
		log.Printf("🔐 OAuth authentication enabled (%s)", s.config.AuthProvider)
	}

	return nil
}

// GetPublicURL returns the public URL of the tunnel
func (s *Service) GetPublicURL() string {
	if s == nil || s.tunnel == nil {
		return ""
	}
	return s.tunnel.URL().String()
}

// Stop stops the ngrok tunnel
func (s *Service) Stop() error {
	if s == nil || s.tunnel == nil {
		return nil
	}

	log.Println("🔌 Stopping ngrok tunnel...")
	return s.tunnel.Close()
}

// Wait waits for the tunnel to close
func (s *Service) Wait() {
	if s == nil || s.tunnel == nil {
		return
	}
	<-s.tunnel.Done()
}
