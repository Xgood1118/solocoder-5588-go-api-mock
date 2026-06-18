package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"apimock/internal/adminapi"
	"apimock/internal/har"
	"apimock/internal/mockserver"
	"apimock/internal/recorder"
	"apimock/internal/scene"
	"apimock/internal/storage"
)

const (
	MockServerPort = ":8081"
	RecorderPort   = ":8082"
)

func main() {
	dataDir := flag.String("data", "./data", "data directory for BoltDB")
	defaultTarget := flag.String("target", "", "default proxy target URL")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	store, err := storage.New(*dataDir)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}
	defer store.Close()

	sceneManager, err := scene.NewManager(store)
	if err != nil {
		log.Fatalf("failed to initialize scene manager: %v", err)
	}

	harProcessor := har.NewProcessor(store)

	mockEngine := gin.Default()
	mockEngine.Use(corsMiddleware())

	adminAPI := adminapi.New(store, sceneManager)
	adminAPI.RegisterRoutes(mockEngine)

	mockSrv := mockserver.New(store, sceneManager)
	mockSrv.RegisterRoutes(mockEngine)

	recorderEngine := gin.Default()
	recorderEngine.Use(corsMiddleware())

	recorderProxy := recorder.NewProxy(store)
	if *defaultTarget != "" {
		if err := recorderProxy.SetTarget(*defaultTarget); err != nil {
			log.Printf("warning: failed to set default target: %v", err)
		}
	}
	recorderProxy.RegisterRoutes(recorderEngine)

	mockServer := &http.Server{
		Addr:    MockServerPort,
		Handler: mockEngine,
	}

	recorderServer := &http.Server{
		Addr:    RecorderPort,
		Handler: recorderEngine,
	}

	go func() {
		log.Printf("Mock server starting on port %s", MockServerPort)
		if err := mockServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("mock server failed: %v", err)
		}
	}()

	go func() {
		log.Printf("Recorder proxy starting on port %s", RecorderPort)
		if err := recorderServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("recorder server failed: %v", err)
		}
	}()

	log.Println("API Mock Server started successfully")
	log.Printf("  - Mock Server: http://localhost%s", MockServerPort)
	log.Printf("  - Admin API:   http://localhost%s/api/admin", MockServerPort)
	log.Printf("  - Recorder:    http://localhost%s", RecorderPort)
	log.Printf("  - HAR Processor: initialized")
	log.Printf("  - Scene Manager: initialized (current: %s)", sceneManager.GetCurrentScene())
	log.Println()
	log.Println("Available scenes:")
	scenes, _ := sceneManager.ListScenes()
	for _, s := range scenes {
		log.Printf("  - %s (%s)", s.ID, s.Name)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mockServer.Shutdown(ctx); err != nil {
		log.Printf("mock server shutdown error: %v", err)
	}
	if err := recorderServer.Shutdown(ctx); err != nil {
		log.Printf("recorder server shutdown error: %v", err)
	}

	log.Println("Servers stopped")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Mock-Scene")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
