package main

import (
	"fmt"
	"log"
	"os"

	"jimeng-service/config"
	"jimeng-service/handlers"
	"jimeng-service/models"
	"jimeng-service/services"

	"github.com/gin-gonic/gin"
)

func main() {
	f, err := os.OpenFile("jimeng-service.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		defer f.Close()
	}

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := os.MkdirAll(cfg.Upload.Path, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	if err := models.LoadTasks(); err != nil {
		log.Printf("Warning: failed to load tasks: %v", err)
	}

	keyPool, err := services.NewKeyPool(&cfg.KeyPool)
	if err != nil {
		log.Fatalf("Failed to initialize key pool: %v", err)
	}

	jimengClient := services.NewJimengClient(&cfg.Jimeng, keyPool)

	apiKeyHandler := handlers.NewAPIKeyHandler(keyPool)
	imageHandler := handlers.NewImageHandler(jimengClient, keyPool)
	videoHandler := handlers.NewVideoHandler(jimengClient, keyPool)
	taskHandler := handlers.NewTaskHandler(cfg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.Static("/uploads", cfg.Upload.Path)
	r.StaticFile("/", "./static/index.html")

	r.GET("/index.html", func(c *gin.Context) {
		c.File("./static/index.html")
	})
	r.GET("/keys.html", func(c *gin.Context) {
		c.File("./static/keys.html")
	})
	r.GET("/image.html", func(c *gin.Context) {
		c.File("./static/image.html")
	})
	r.GET("/video.html", func(c *gin.Context) {
		c.File("./static/video.html")
	})
	r.GET("/tasks.html", func(c *gin.Context) {
		c.File("./static/tasks.html")
	})

	r.Static("/static/css", "./static/css")
	r.Static("/static/js", "./static/js")

	api := r.Group("/api")
	{
		keys := api.Group("/keys")
		{
			keys.GET("", apiKeyHandler.GetKeys)
			keys.POST("", apiKeyHandler.AddKey)
			keys.PUT("/:id", apiKeyHandler.UpdateKey)
			keys.DELETE("/:id", apiKeyHandler.DeleteKey)
			keys.POST("/import", apiKeyHandler.ImportKeys)
			keys.GET("/status", apiKeyHandler.GetStatus)
			keys.POST("/:id/reset", apiKeyHandler.ResetKey)
		}

		image := api.Group("/image")
		{
			image.POST("/generate", imageHandler.Generate)
			image.GET("/functions", imageHandler.GetFunctions)
			image.GET("/size-presets", imageHandler.GetSizePresets)
		}

		video := api.Group("/video")
		{
			video.POST("/generate", videoHandler.Generate)
			video.GET("/functions", videoHandler.GetFunctions)
			video.GET("/aspect-ratios", videoHandler.GetAspectRatios)
			video.GET("/camera-templates", videoHandler.GetCameraTemplates)
			video.GET("/frame-options", videoHandler.GetFrameOptions)
		}

		task := api.Group("/task")
		{
			task.GET("/status/:id", taskHandler.GetStatus)
			task.GET("/result/:id", taskHandler.GetResult)
			task.GET("/list", taskHandler.GetTasks)
			task.DELETE("/:id", taskHandler.DeleteTask)
		}

		api.POST("/upload", taskHandler.Upload)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	log.Printf("Open http://localhost:%d in your browser", cfg.Server.Port)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
