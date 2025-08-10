package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProcessTask represents a queued or running process/AI task
// ID should be unique (e.g., timestamp+pid+random)
type ProcessTask struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // e.g. "ai", "command"
	User      string    `json:"user"`
	ChatID    int64     `json:"chat_id"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // e.g. "queued", "running", "done", "error"
	Info      string    `json:"info"`
}

type processQueue struct {
	Tasks []ProcessTask `json:"tasks"`
}

var processLock sync.Mutex

func processFilePath() string {
	return filepath.Join(GetAppDir(), "process.json")
}

// LoadProcessQueue loads the process queue from disk
func LoadProcessQueue() (*processQueue, error) {
	processLock.Lock()
	defer processLock.Unlock()
	path := processFilePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &processQueue{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var q processQueue
	if err := json.NewDecoder(f).Decode(&q); err != nil {
		return &processQueue{}, nil
	}
	return &q, nil
}

// SaveProcessQueue saves the process queue to disk
func SaveProcessQueue(q *processQueue) error {
	processLock.Lock()
	defer processLock.Unlock()
	path := processFilePath()
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(q); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AddProcessTask adds a new task to the queue
func AddProcessTask(task ProcessTask) error {
	q, err := LoadProcessQueue()
	if err != nil {
		return err
	}
	q.Tasks = append(q.Tasks, task)
	return SaveProcessQueue(q)
}

// RemoveProcessTask removes a task by ID
func RemoveProcessTask(id string) error {
	q, err := LoadProcessQueue()
	if err != nil {
		return err
	}
	newTasks := make([]ProcessTask, 0, len(q.Tasks))
	for _, t := range q.Tasks {
		if t.ID != id {
			newTasks = append(newTasks, t)
		}
	}
	q.Tasks = newTasks
	return SaveProcessQueue(q)
}

// GetActiveTasks returns all tasks not marked as done or error
func GetActiveTasks() ([]ProcessTask, error) {
	q, err := LoadProcessQueue()
	if err != nil {
		return nil, err
	}
	active := []ProcessTask{}
	for _, t := range q.Tasks {
		if t.Status != "done" && t.Status != "error" {
			active = append(active, t)
		}
	}
	return active, nil
}
