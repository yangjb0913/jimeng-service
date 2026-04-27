package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"jimeng-service/config"
	"jimeng-service/models"
	"jimeng-service/services"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	jimengClient *services.JimengClient
	keyPool      *services.KeyPool
}

func NewVideoHandler(jimengClient *services.JimengClient, keyPool *services.KeyPool) *VideoHandler {
	return &VideoHandler{
		jimengClient: jimengClient,
		keyPool:      keyPool,
	}
}

type GenerateVideoRequest struct {
	Function      string   `json:"function" binding:"required"`
	Prompt        string   `json:"prompt" binding:"required"`
	ImageURLs     []string `json:"image_urls"`
	Frames        int      `json:"frames" binding:"required"`
	AspectRatio   string   `json:"aspect_ratio"`
	TemplateID    string   `json:"template_id"`
	CameraStrength string  `json:"camera_strength"`
	Seed          int      `json:"seed"`
}

func (h *VideoHandler) Generate(c *gin.Context) {
	var req GenerateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	if req.Frames != 121 && req.Frames != 241 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "frames must be 121 (5s) or 241 (10s)",
		})
		return
	}

	duration := services.ParseVideoDuration(req.Frames)

	apiKey, err := h.keyPool.SelectKey(req.Function)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if pass, msg := apiKey.CheckVideoQuota(duration); !pass {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": msg,
		})
		return
	}

	extra := make(map[string]interface{})
	extra["frames"] = req.Frames
	if req.AspectRatio != "" {
		extra["aspect_ratio"] = req.AspectRatio
	}
	if req.TemplateID != "" {
		extra["template_id"] = req.TemplateID
	}
	if req.CameraStrength != "" {
		extra["camera_strength"] = req.CameraStrength
	}
	if req.Seed != 0 {
		extra["seed"] = req.Seed
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

	task := models.CreateTask(taskID, internalTaskID, "video", req.Function, req.Prompt)
	task.Status = string(models.TaskStatusInQueue)
	task.Duration = duration
	models.AddTask(task)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"task_id":  taskID,
			"status":   task.Status,
			"duration": duration,
		},
	})

	go h.pollVideoResult(taskID, internalTaskID, req.Function, apiKey, duration)
}

func (h *VideoHandler) pollVideoResult(taskID, internalTaskID, function string, apiKey *models.APIKey, duration int) {
	maxAttempts := 300
	attempt := 0

	for attempt < maxAttempts {
		resp, err := h.jimengClient.QueryTask(internalTaskID, function, apiKey)
		if err != nil {
			models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, err.Error())
			return
		}

		switch resp.Data.Status {
		case "done":
			videoURL := resp.Data.VideoURL

			if videoURL != "" {
				cfg := config.Get()
				filename := fmt.Sprintf("%s.mp4", taskID)
				filePath := filepath.Join(cfg.Upload.Path, filename)

				if err := h.downloadVideo(videoURL, filePath); err == nil {
					videoURL = "/uploads/" + filename
				}
			}

			models.UpdateTaskResult(taskID, string(models.TaskStatusDone), &models.TaskResult{
				VideoURL: videoURL,
			}, "")

			apiKey.UseVideoQuota(duration)
			return

		case "failed", "not_found", "expired":
			models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, resp.Data.Status)
			return
		}

		attempt++
		time.Sleep(2 * time.Second)
	}

	models.UpdateTaskResult(taskID, string(models.TaskStatusFailed), nil, "timeout")
}

func (h *VideoHandler) downloadVideo(videoURL, filePath string) error {
	resp, err := http.Get(videoURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func (h *VideoHandler) GetFunctions(c *gin.Context) {
	functions := []map[string]interface{}{
		{"key": "t2v_720", "name": "文生视频720p", "type": "video", "frames": []int{121, 241}},
		{"key": "t2v_1080", "name": "文生视频1080p", "type": "video", "frames": []int{121, 241}},
		{"key": "i2v_first_720", "name": "首帧视频720p", "type": "video", "frames": []int{121, 241}},
		{"key": "i2v_first_1080", "name": "首帧视频1080p", "type": "video", "frames": []int{121, 241}},
		{"key": "i2v_first_tail_720", "name": "首尾帧视频720p", "type": "video", "frames": []int{121, 241}},
		{"key": "i2v_first_tail_1080", "name": "首尾帧视频1080p", "type": "video", "frames": []int{121, 241}},
		{"key": "i2v_recamera_720", "name": "运镜视频720p", "type": "video", "frames": []int{121, 241}},
		{"key": "ti2v_pro", "name": "3.0Pro视频", "type": "video", "frames": []int{121, 241}},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": functions,
	})
}

func (h *VideoHandler) GetAspectRatios(c *gin.Context) {
	ratios := []map[string]string{
		{"key": "16:9", "label": "16:9 (横屏)"},
		{"key": "4:3", "label": "4:3"},
		{"key": "1:1", "label": "1:1 (方形)"},
		{"key": "3:4", "label": "3:4 (竖屏)"},
		{"key": "9:16", "label": "9:16 (竖屏)"},
		{"key": "21:9", "label": "21:9 (宽屏)"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": ratios,
	})
}

func (h *VideoHandler) GetCameraTemplates(c *gin.Context) {
	templates := []map[string]string{
		{"key": "hitchcock_dolly_in", "name": "希区柯克推进"},
		{"key": "hitchcock_dolly_out", "name": "希区柯克拉远"},
		{"key": "robo_arm", "name": "机械臂"},
		{"key": "dynamic_orbit", "name": "动感环绕"},
		{"key": "central_orbit", "name": "中心环绕"},
		{"key": "crane_push", "name": "起重机"},
		{"key": "quick_pull_back", "name": "超级拉远"},
		{"key": "counterclockwise_swivel", "name": "逆时针回旋"},
		{"key": "clockwise_swivel", "name": "顺时针回旋"},
		{"key": "handheld", "name": "手持运镜"},
		{"key": "rapid_push_pull", "name": "快速推拉"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": templates,
	})
}

func (h *VideoHandler) GetFrameOptions(c *gin.Context) {
	options := []map[string]interface{}{
		{"frames": 121, "duration": 5, "label": "5秒"},
		{"frames": 241, "duration": 10, "label": "10秒"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": options,
	})
}
