package internal

import (
	"fmt"
	"log"
	"time"
)

type Task struct {
	ID          int64
	ChatID      int64
	Title       string
	Description string
	DueAt       time.Time
	Status      string // pending, completed, cancelled
	Reminded    bool
	CreatedAt   time.Time
}

func SaveTask(t Task) error {
	_, err := DB.Exec(`
		INSERT INTO tasks (chat_id, title, description, due_at, status)
		VALUES (?, ?, ?, ?, ?)`,
		t.ChatID, t.Title, t.Description, t.DueAt, "pending")
	return err
}

func GetPendingTasks() ([]Task, error) {
	rows, err := DB.Query(`SELECT id, chat_id, title, description, due_at, status, reminded, created_at FROM tasks WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(&t.ID, &t.ChatID, &t.Title, &t.Description, &t.DueAt, &t.Status, &t.Reminded, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func MarkTaskReminded(id int64) error {
	_, err := DB.Exec(`UPDATE tasks SET reminded = 1 WHERE id = ?`, id)
	return err
}

// Scheduler represents the background task processor
type Scheduler struct {
	NotifyFunc func(chatID int64, message string)
}

func (s *Scheduler) Start() {
	log.Println("Starting Background Scheduler...")
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			s.CheckTasks()
		}
	}()
}

func (s *Scheduler) CheckTasks() {
	tasks, err := GetPendingTasks()
	if err != nil {
		log.Printf("Scheduler error: %v", err)
		return
	}

	now := time.Now()
	for _, t := range tasks {
		if !t.Reminded && t.DueAt.Before(now.Add(5 * time.Minute)) {
			msg := fmt.Sprintf("⏰ **Task Reminder**\n**Title:** %s\n**Description:** %s\n**Due:** %s", 
				t.Title, t.Description, t.DueAt.Format("2006-01-02 15:04"))
			
			if s.NotifyFunc != nil {
				s.NotifyFunc(t.ChatID, msg)
				MarkTaskReminded(t.ID)
			}
		}
	}
}
