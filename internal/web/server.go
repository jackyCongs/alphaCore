package web

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"alphacore/internal/models"
	"github.com/gin-gonic/gin"
)

type Server struct {
	stateManager *StateManager
	router       *gin.Engine

	// SSE client management
	mu      sync.Mutex
	clients map[chan string]bool
}

func NewServer(sm *StateManager) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New() // Use gin.New() instead of gin.Default() to skip logging middleware for performance
	router.Use(gin.Recovery())

	s := &Server{
		stateManager: sm,
		router:       router,
		clients:      make(map[chan string]bool),
	}

	// Serve static files
	router.Static("/static", "./internal/web/static")
	router.GET("/", func(c *gin.Context) {
		if c.Query("type") == "json" {
			c.JSON(http.StatusOK, s.stateManager.GetAllSorted())
			return
		}
		c.File("./internal/web/static/index.html")
	})

	// SSE stream
	router.GET("/stream", s.streamHandler)

	// API to get initial snapshot
	router.GET("/api/snapshot", func(c *gin.Context) {
		c.JSON(http.StatusOK, s.stateManager.GetAllSorted())
	})

	// Start a single goroutine that pushes snapshots to all SSE clients at a fixed interval
	go s.broadcastLoop()

	return s
}

// PushUpdate is called by the MQ client whenever a new batch arrives.
// Now it ONLY updates the state — no direct SSE push. The broadcastLoop handles delivery.
func (s *Server) PushUpdate(batch []models.IndexResult) {
	s.stateManager.UpdateBatch(batch)
}

// broadcastLoop sends the full sorted snapshot to all SSE clients every 500ms.
// This is the single most important optimization: no matter how fast ticks arrive,
// the browser only receives 2 updates per second.
func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		clientCount := len(s.clients)
		s.mu.Unlock()

		if clientCount == 0 {
			continue
		}

		snapshot := s.stateManager.GetAllSorted()
		data, err := json.Marshal(snapshot)
		if err != nil {
			continue
		}
		msg := string(data)

		s.mu.Lock()
		for ch := range s.clients {
			select {
			case ch <- msg:
			default:
				// Client is slow, drop this frame
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) streamHandler(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	clientChan := make(chan string, 2) // Very small buffer — we want to drop stale frames, not queue them

	s.mu.Lock()
	s.clients[clientChan] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, clientChan)
		close(clientChan)
		s.mu.Unlock()
	}()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			c.Writer.Write([]byte("data: " + msg + "\n\n"))
			c.Writer.Flush()
		}
	}
}

func (s *Server) Run(addr string) error {
	log.Printf("🌐 Web 仪表盘启动: http://localhost%s", addr)
	return s.router.Run(addr)
}
