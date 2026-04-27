package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"jimeng-service/config"
	"jimeng-service/models"
	"jimeng-service/services"

	"github.com/gin-gonic/gin"
)

type ImageHandler struct {
	jimengClient *services.JimengClient
	keyPool      *services.KeyPool
}

func NewImageHandler(jimengClient *services.JimengClient, keyPool *services.KeyPool) *ImageHandler {
	return &ImageHandler{
		jimengClient: jimengClient,
		keyPool:      keyPool,
	}
}

type GenerateImageRequest struct {
	Function    string   `json:"function" binding:"required"`
	Prompt      string   `json:"prompt" binding:"required"`
	ImageURLs   []string `json:"image_urls"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Scale       float64  `json:"scale"`
	ForceSingle bool     `json:"force_single"`
}

func (h *ImageHandler) Generate(c *gin.Context) {
	var req GenerateImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	apiKey, err := h.keyPool.SelectKey(req.Function)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if pass, msg := apiKey.CheckImageQuota(1); !pass {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": msg,
		})
		return
	}

	extra := make(map[string]interface{})
	if req.Width > 0 && req.Height > 0 {
		extra["width"] = req.Width
		extra["height"] = req.Height
	}
	if req.Scale > 0 {
		extra["scale"] = req.Scale
	}
	if req.ForceSingle {
		extra["force_single"] = true
	}

	taskID := services.GenerateTaskID()
	internalTaskID, _, err := h.jimengClient.SubmitTask(req.Function, req.Prompt, req.ImageURLs, extra)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	task := models.CreateTask(taskID, internalTaskID, "image", req.Function, req.Prompt)
	task.Status = string(models.TaskStatusInQueue)
	models.AddTask(task)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"task_id": taskID,
			"status":  task.Status,
		},
	})

	go h.pollImageResult(taskID, internalTaskID, req.Function, apiKey)
}

func (h *ImageHandler) pollImageResult(taskID, internalTaskID, function string, apiKey *models.APIKey) {
	maxAttempts := 120
	attempt := 0

	for attempt < maxAttempts {
		resp, err := h.jimengClient.QueryTask(internalTaskID, function, apiKey)
		if err != nil {
			models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, err.Error())
			return
		}

		switch resp.Data.Status {
		case "done":
			var imageURLs []string

			if len(resp.Data.BinaryDataBase64) > 0 {
				cfg := config.Get()
				for i, data := range resp.Data.BinaryDataBase64 {
					filename := fmt.Sprintf("%s_%d.png", taskID, i)
					filePath := filepath.Join(cfg.Upload.Path, filename)

					decoded, err := base64.StdEncoding.DecodeString(data)
					if err != nil {
						continue
					}

					if err := os.WriteFile(filePath, decoded, 0644); err == nil {
						imageURLs = append(imageURLs, "/uploads/"+filename)
					}
				}
			}

			if len(imageURLs) == 0 && len(resp.Data.ImageURLs) > 0 {
				imageURLs = resp.Data.ImageURLs
			}

			models.UpdateTaskResult(taskID, string(models.TaskStatusDone), &models.TaskResult{
				ImageURLs: imageURLs,
			}, "")

			h.keyPool.GetKeyByID(apiKey.ID)
			apiKey.UseImageQuota(1)
			return

		case "failed", "not_found", "expired":
			models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, resp.Data.Status)
			return
		}

		attempt++
		time.Sleep(3 * time.Second)
	}

	models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, "timeout")
}

func (h *ImageHandler) GetFunctions(c *gin.Context) {
	functions := []map[string]interface{}{
		{"key": "t2i_v40", "name": "文生图4.0", "type": "image"},
		{"key": "t2i_46", "name": "生图4.6", "type": "image"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": functions,
	})
}

func (h *ImageHandler) GetSizePresets(c *gin.Context) {
	presets := []map[string]interface{}{
		{"width": 1024, "height": 1024, "label": "1K (1:1)"},
		{"width": 2048, "height": 2048, "label": "2K (1:1)"},
		{"width": 2304, "height": 1728, "label": "2K (4:3)"},
		{"width": 2560, "height": 1440, "label": "2K (16:9)"},
		{"width": 4096, "height": 4096, "label": "4K (1:1)"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": presets,
	})
}
