package deadman

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/guardian/agent/internal/hardening"
)

type TrialJob struct {
	JobID      string            `json:"jobId"`
	Key        string            `json:"key"`
	RollbackAt time.Time         `json:"rollbackAt"`
	Files      map[string]string `json:"files"`
}

type Deadman struct {
	path string
	mu   sync.Mutex
}

func New() *Deadman {
	dir := os.Getenv("GUARDIAN_STATE_DIR")
	if dir == "" {
		dir = "/var/lib/guardian"
	}
	return &Deadman{path: filepath.Join(dir, "trial_job.json")}
}

func (d *Deadman) SetTrial(job TrialJob) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.path, data, 0o600)
}

func (d *Deadman) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = os.Remove(d.path)
}

func (d *Deadman) GetActive() (*TrialJob, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := os.ReadFile(d.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job TrialJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// StartMonitor 启动本地死人开关监控后台循环。
// 在启动时，如果读取到本地有残留的 trial_job 且已超时，会立刻触发自主回滚。
func (d *Deadman) StartMonitor(ctx context.Context, exec *hardening.Executor) {
	// 启动自检
	if job, err := d.GetActive(); err == nil && job != nil {
		if time.Now().After(job.RollbackAt) {
			log.Printf("[deadman] Found stale trial job %s (key: %s) on startup that has timed out. Triggering autonomous rollback!", job.JobID, job.Key)
			if err := exec.Rollback(ctx, job.Files); err == nil {
				d.Clear()
			} else {
				log.Printf("[deadman] Autonomous rollback failed on startup: %v", err)
			}
		} else {
			left := time.Until(job.RollbackAt)
			log.Printf("[deadman] Resuming active trial job %s (key: %s) with %s left before auto-rollback", job.JobID, job.Key, left.Round(time.Second))
		}
	}

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				job, err := d.GetActive()
				if err != nil || job == nil {
					continue
				}
				if time.Now().After(job.RollbackAt) {
					log.Printf("[deadman] Confirm timeout reached for trial job %s (key: %s). Initiating local autonomous rollback!", job.JobID, job.Key)
					if err := exec.Rollback(ctx, job.Files); err == nil {
						log.Printf("[deadman] Autonomous rollback succeeded. Connection settings reverted.")
						d.Clear()
					} else {
						log.Printf("[deadman] Autonomous rollback failed: %v", err)
					}
				}
			}
		}
	}()
}
