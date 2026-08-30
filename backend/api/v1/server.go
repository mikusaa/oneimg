package v1

import "oneimg/backend/services"

type Server struct {
	services *services.Services
	limiter  *rateLimiter
}

func NewServer(services *services.Services) *Server {
	return &Server{services: services, limiter: newRateLimiter()}
}
