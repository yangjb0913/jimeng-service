package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"jimeng-service/config"
	"jimeng-service/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TaskHandler struct {
	cfg *config.Config
}

func NewTaskHandler(cfg *config.Config) *TaskHandler {
	return &TaskHandler{cfg: cfg}
}

func (h *TaskHandler) GetStatus(c *gin.Context) {
	taskID := c.Param("id")

	task, found := models.GetTask(taskID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "task not found",
		})
		return
	}

	response := gin.H{
		"code": 0,
		"data": gin.H{
			"task_id":    task.ID,
			"status":     task.Status,
			"type":       task.Type,
			"function":   task.Function,
			"prompt":     task.Prompt,
			"created_at": task.CreatedAt,
		},
	}

	if task.CompletedAt != nil {
		response["data"].(gin.H)["completed_at"] = task.CompletedAt
	}

	if task.Duration > 0 {
		response["data"].(gin.H)["duration"] = task.Duration
	}

	if task.Error != "" {
		response["data"].(gin.H)["error"] = task.Error
	}

	c.JSON(http.StatusOK, response)
}

func (h *TaskHandler) GetResult(c *gin.Context) {
	taskID := c.Param("id")

	task, found := models.GetTask(taskID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "task not found",
		})
		return
	}

	response := gin.H{
		"code": 0,
		"data": gin.H{
			"task_id": task.ID,
			"status":  task.Status,
		},
	}

	if task.Status == string(models.TaskStatusDone) && task.Result != nil {
		response["data"].(gin.H)["result"] = task.Result
	}

	if task.Error != "" {
		response["data"].(gin.H)["error"] = task.Error
	}

	c.JSON(http.StatusOK, response)
}

func (h *TaskHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "no file uploaded: " + err.Error(),
		})
		return
	}

	if file.Size > h.cfg.Upload.MaxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": fmt.Sprintf("file size exceeds limit (%dMB)", h.cfg.Upload.MaxSize/1024/1024),
		})
		return
	}

	ext := filepath.Ext(file.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "only jpg and png files are allowed",
		})
		return
	}

	newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	savePath := filepath.Join(h.cfg.Upload.Path, newFilename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "failed to save file: " + err.Error(),
		})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	fileURL := fmt.Sprintf("%s://%s/uploads/%s", scheme, host, newFilename)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"url":      fileURL,
			"filename": newFilename,
			"size":     file.Size,
		},
	})
}

func (h *TaskHandler) GetTasks(c *gin.Context) {
	taskID := c.Query("task_id")
	taskType := c.Query("type")
	status := c.Query("status")
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var allTasks []*models.Task
	var filteredTasks []*models.Task

	allTasksMap := models.GetAllTasks()
	for _, t := range allTasksMap {
		allTasks = append(allTasks, t)
	}

	sort.Slice(allTasks, func(i, j int) bool {
		return allTasks[i].CreatedAt.After(allTasks[j].CreatedAt)
	})

	for _, t := range allTasks {
		if taskID != "" && t.ID != taskID {
			continue
		}
		if taskType != "" && t.Type != taskType {
			continue
		}
		if status != "" && t.Status != status {
			continue
		}
		filteredTasks = append(filteredTasks, t)
	}

	if offset >= len(filteredTasks) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"tasks":  []models.Task{},
				"total": len(filteredTasks),
				"limit": limit,
				"offset": offset,
			},
		})
		return
	}

	end := offset + limit
	if end > len(filteredTasks) {
		end = len(filteredTasks)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"tasks":  filteredTasks[offset:end],
			"total": len(filteredTasks),
			"limit": limit,
			"offset": offset,
		},
	})
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	task, found := models.GetTask(taskID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Task not found",
		})
		return
	}

	var filesToDelete []string
	cfg := config.Get()
	if task.Result != nil {
		for _, url := range task.Result.ImageURLs {
			if strings.HasPrefix(url, "/uploads/") {
				filesToDelete = append(filesToDelete, filepath.Join(cfg.Upload.Path, strings.TrimPrefix(url, "/uploads/")))
			}
		}
		if task.Result.VideoURL != "" && strings.HasPrefix(task.Result.VideoURL, "/uploads/") {
			filesToDelete = append(filesToDelete, filepath.Join(cfg.Upload.Path, strings.TrimPrefix(task.Result.VideoURL, "/uploads/")))
		}
	}

	models.DeleteTask(taskID)

	for _, filePath := range filesToDelete {
		os.Remove(filePath)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Task deleted",
	})
}
