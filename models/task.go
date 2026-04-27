package models

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Task struct {
	ID           string     `json:"id"`
	TaskID       string     `json:"task_id"`
	Type         string     `json:"type"`
	Function     string     `json:"function"`
	Status       string     `json:"status"`
	Prompt       string     `json:"prompt"`
	Result       *TaskResult `json:"result,omitempty"`
	Error        string     `json:"error,omitempty"`
	Duration     int        `json:"duration,omitempty"`
	ImageCount   int        `json:"image_count,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type TaskResult struct {
	ImageURLs []string `json:"image_urls,omitempty"`
	VideoURL  string   `json:"video_url,omitempty"`
}

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusInQueue  TaskStatus = "in_queue"
	TaskStatusGenerating TaskStatus = "generating"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusFailed    TaskStatus = "failed"
)

var (
	taskStore  = make(map[string]*Task)
	taskMutex  sync.RWMutex
	taskFile   = "tasks.json"
)

func SetTaskFile(filename string) {
	taskFile = filename
}

func CreateTask(id, taskID, taskType, function, prompt string) *Task {
	task := &Task{
		ID:        id,
		TaskID:    taskID,
		Type:      taskType,
		Function:  function,
		Status:    string(TaskStatusPending),
		Prompt:    prompt,
		CreatedAt: time.Now(),
	}
	taskMutex.Lock()
	taskStore[id] = task
	taskMutex.Unlock()
	go saveTasks()
	return task
}

func GetTask(id string) (*Task, bool) {
	taskMutex.RLock()
	defer taskMutex.RUnlock()
	t, ok := taskStore[id]
	return t, ok
}

func UpdateTaskResult(id string, status string, result *TaskResult, errMsg string) {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	if task, ok := taskStore[id]; ok {
		task.Status = status
		task.Result = result
		task.Error = errMsg
		if status == string(TaskStatusDone) || status == string(TaskStatusFailed) {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	go saveTasks()
}

func SetTaskDuration(id string, duration int) {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	if task, ok := taskStore[id]; ok {
		task.Duration = duration
	}
}

func SetTaskImageCount(id string, count int) {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	if task, ok := taskStore[id]; ok {
		task.ImageCount = count
	}
}

func GetAllTasks() map[string]*Task {
	taskMutex.RLock()
	defer taskMutex.RUnlock()
	result := make(map[string]*Task)
	for k, v := range taskStore {
		result[k] = v
	}
	return result
}

func AddTask(task *Task) {
	taskMutex.Lock()
	taskStore[task.ID] = task
	taskMutex.Unlock()
	go saveTasks()
}

func LoadTasks() error {
	taskMutex.Lock()
	defer taskMutex.Unlock()

	data, err := os.ReadFile(taskFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		taskStore[task.ID] = task
	}

	return nil
}

func saveTasks() error {
	taskMutex.RLock()
	tasks := make([]*Task, 0, len(taskStore))
	for _, task := range taskStore {
		tasks = append(tasks, task)
	}
	taskMutex.RUnlock()

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(taskFile, data, 0644)
}

func DeleteTask(id string) bool {
	taskMutex.Lock()
	defer taskMutex.Unlock()
	if _, ok := taskStore[id]; ok {
		delete(taskStore, id)
		go saveTasks()
		return true
	}
	return false
}
