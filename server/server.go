package server

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	tasks   *TaskStore
	clients *ClientManager
	router  *gin.Engine
}

func New() *Server {
	s := &Server{
		tasks:   NewTaskStore(),
		clients: NewClientManager(),
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.POST("/task", HandleCreateTask(s))
		api.GET("/task/:id", HandleGetTask(s))
	}

	r.GET("/ws", HandleWebSocket(s))

	s.router = r

	// Start timeout checker
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.tasks.TimeoutExpired(120 * time.Second)
		}
	}()

	return s
}

func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
