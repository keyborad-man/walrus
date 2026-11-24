package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"walrus/internal/apperr"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/runtime"
	"walrus/internal/steam"
)

type CronService struct {
	store      *db.Store
	fs         *dst.FS
	driver     runtime.Driver
	clusterSvc *ClusterService
	backupSvc  *BackupService
	steamMgr   *steam.Manager
	webhookSvc *WebhookService

	mu     sync.Mutex
	engine *cron.Cron
	parser cron.Parser
}

func NewCronService(
	store *db.Store,
	fs *dst.FS,
	driver runtime.Driver,
	clusterSvc *ClusterService,
	backupSvc *BackupService,
	steamMgr *steam.Manager,
	webhookSvc *WebhookService,
) *CronService {
	p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return &CronService{
		store:      store,
		fs:         fs,
		driver:     driver,
		clusterSvc: clusterSvc,
		backupSvc:  backupSvc,
		steamMgr:   steamMgr,
		webhookSvc: webhookSvc,
		parser:     p,
	}
}

func (s *CronService) ValidateExpr(expr string) error {
	_, err := s.parser.Parse(strings.TrimSpace(expr))
	return err
}

func (s *CronService) Start() error {
	return s.Reload()
}

func (s *CronService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine != nil {
		ctx := s.engine.Stop()
		<-ctx.Done()
		s.engine = nil
	}
}

func (s *CronService) Reload() error {
	tasks, err := s.store.ListScheduledTasks("")
	if err != nil {
		return err
	}

	next := cron.New(cron.WithParser(s.parser))
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		taskID := t.ID
		if _, err := next.AddFunc(t.CronExpr, func() {
			s.executeTaskByID(taskID, false)
		}); err != nil {
			slog.Warn("skipping invalid cron expression", "task_id", t.ID, "expr", t.CronExpr, "err", err)
		}
	}
	next.Start()

	s.mu.Lock()
	old := s.engine
	s.engine = next
	s.mu.Unlock()

	if old != nil {
		old.Stop()
	}
	return nil
}

func (s *CronService) RunNow(taskID int64) error {
	if _, err := s.store.GetScheduledTask(taskID); err != nil {
		return err
	}
	go s.executeTaskByID(taskID, true)
	return nil
}

func (s *CronService) executeTaskByID(taskID int64, isManualRun bool) {
	t, err := s.store.GetScheduledTask(taskID)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	var params map[string]any
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)
	if params == nil {
		params = map[string]any{}
	}

	var runErr error
	var summary string

	switch t.TaskType {
	case "backup":
		summary, runErr = s.runBackupTask(ctx, t, params)
	case "announce":
		summary, runErr = s.runAnnounceTask(t, params)
	case "restart":
		summary, runErr = s.runRestartTask(ctx, t, params, isManualRun)
	case "update":
		summary, runErr = s.runUpdateTask(ctx, t, params)
	default:
		runErr = apperr.Newf(apperr.CodeInvalidTaskType, map[string]any{"task_type": t.TaskType}, "unknown task type %q", t.TaskType)
	}

	if runErr != nil {
		status := "error: " + runErr.Error()
		s.store.RecordScheduledTaskRun(t.ID, status)
		if s.webhookSvc != nil {
			s.webhookSvc.Dispatch("cron.failed", "Scheduled Task Failed: "+t.Name, runErr.Error(), t.ClusterID)
		}
		slog.Warn("scheduled task failed", "id", t.ID, "name", t.Name, "err", runErr)
		return
	}
	if summary == "" {
		summary = "ok"
	}
	s.store.RecordScheduledTaskRun(t.ID, summary)
}

func (s *CronService) targetClusters(clusterID string) ([]string, error) {
	if clusterID == "" || clusterID == "*" {
		return s.fs.ListClusterIDs()
	}
	return []string{clusterID}, nil
}

func (s *CronService) runBackupTask(ctx context.Context, t *db.ScheduledTask, params map[string]any) (string, error) {
	targets, err := s.targetClusters(t.ClusterID)
	if err != nil {
		return "", err
	}
	note, _ := params["note"].(string)
	if note == "" {
		note = "Scheduled: " + t.Name
	}
	done := 0
	for _, cid := range targets {
		rec, bErr := s.backupSvc.CreateBackup(ctx, cid, "scheduled", note, "cron")
		if bErr != nil {
			if s.webhookSvc != nil {
				s.webhookSvc.Dispatch("backup.failed", "Scheduled Backup Failed", bErr.Error(), cid)
			}
			return "", bErr
		}
		done++
		if s.webhookSvc != nil {
			s.webhookSvc.Dispatch("backup.complete", "Scheduled Backup Completed", fmt.Sprintf("Archive %s created (%d bytes)", rec.Filename, rec.SizeBytes), cid)
		}
	}
	return fmt.Sprintf("ok (%d backup(s))", done), nil
}

func (s *CronService) runAnnounceTask(t *db.ScheduledTask, params map[string]any) (string, error) {
	msg, _ := params["message"].(string)
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "", apperr.New(apperr.CodeAnnounceMessageEmpty, "announcement message is empty")
	}
	targets, err := s.targetClusters(t.ClusterID)
	if err != nil {
		return "", err
	}
	clean := strings.ReplaceAll(msg, `"`, `\"`)
	sent := 0
	for _, cid := range targets {
		if st := s.driver.GetShardStatus(cid, "master"); st.State == runtime.StateRunning {
			if err := s.driver.SendCommand(cid, "master", fmt.Sprintf(`c_announce("%s")`, clean)); err == nil {
				sent++
			}
		}
	}
	return fmt.Sprintf("ok (sent to %d running cluster(s))", sent), nil
}

func (s *CronService) runRestartTask(ctx context.Context, t *db.ScheduledTask, params map[string]any, isManualRun bool) (string, error) {
	targets, err := s.targetClusters(t.ClusterID)
	if err != nil {
		return "", err
	}
	countdownSec := 60
	if v, ok := params["countdown_sec"].(float64); ok && v >= 0 {
		countdownSec = int(v)
	}
	if isManualRun && countdownSec > 10 {
		countdownSec = 5
	}
	noticeTemplate, _ := params["message"].(string)
	if strings.TrimSpace(noticeTemplate) == "" {
		noticeTemplate = "Server scheduled restart in %ds!"
	}

	var runningTargets []string
	for _, cid := range targets {
		if st := s.driver.GetShardStatus(cid, "master"); st.State == runtime.StateRunning {
			runningTargets = append(runningTargets, cid)
		}
	}
	if len(runningTargets) == 0 {
		return "skipped (no running clusters)", nil
	}

	// Send staged countdown announcements.
	if countdownSec > 0 {
		broadcast := func(rem int) {
			txt := strings.ReplaceAll(fmt.Sprintf(noticeTemplate, rem), `"`, `\"`)
			for _, cid := range runningTargets {
				_ = s.driver.SendCommand(cid, "master", fmt.Sprintf(`c_announce("%s")`, txt))
			}
		}
		if countdownSec > 60 {
			broadcast(countdownSec)
			select {
			case <-time.After(time.Duration(countdownSec-60) * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			countdownSec = 60
		}
		if countdownSec > 10 {
			broadcast(countdownSec)
			select {
			case <-time.After(time.Duration(countdownSec-10) * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			countdownSec = 10
		}
		broadcast(countdownSec)
		select {
		case <-time.After(time.Duration(countdownSec) * time.Second):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	for _, cid := range runningTargets {
		if err := s.clusterSvc.RestartCluster(ctx, cid); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("ok (restarted %d cluster(s))", len(runningTargets)), nil
}

func (s *CronService) runUpdateTask(ctx context.Context, t *db.ScheduledTask, params map[string]any) (string, error) {
	branch := "public"
	if st, err := s.store.GetSystemSettings(false); err == nil && st.DSTBranch != "" {
		branch = st.DSTBranch
	}

	st, err := s.steamMgr.CheckRemoteVersion(ctx, branch)
	if err != nil {
		return "", err
	}
	if !st.UpdateNeeded {
		return "up to date (" + st.LocalVersion + ")", nil
	}
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch("steam.update_available", "New DST Server Update Available", fmt.Sprintf("Local %s -> Latest %s (%s)", st.LocalVersion, st.RemoteVersion, branch), "*")
	}

	onlyWhenEmpty := true
	if v, ok := params["only_when_empty"].(bool); ok {
		onlyWhenEmpty = v
	}

	ids, _ := s.fs.ListClusterIDs()
	var running []string
	for _, cid := range ids {
		if s.driver.GetShardStatus(cid, "master").State == runtime.StateRunning {
			running = append(running, cid)
			if onlyWhenEmpty && len(s.driver.GetWorldInfo(cid).OnlinePlayers) > 0 {
				return "deferred (players online)", nil
			}
		}
	}

	for _, cid := range running {
		_ = s.driver.SendCommand(cid, "master", `c_announce("Updating DST server to new build, restarting shortly...")`)
		_ = s.clusterSvc.StopCluster(ctx, cid)
	}

	if err := s.steamMgr.UpdateDSTServer(branch, true); err != nil {
		return "", err
	}

	// Wait for SteamCMD task to finish.
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(1 * time.Second):
			cur := s.steamMgr.Status(branch)
			if cur.TaskState == steam.TaskSuccess {
				for _, cid := range running {
					_ = s.clusterSvc.StartCluster(ctx, cid)
				}
				if s.webhookSvc != nil {
					s.webhookSvc.Dispatch("steam.update_complete", "DST Server Updated", fmt.Sprintf("Updated to build %s and restarted %d cluster(s)", cur.LocalVersion, len(running)), "*")
				}
				return fmt.Sprintf("updated to %s", cur.LocalVersion), nil
			}
			if cur.TaskState == steam.TaskFailed {
				return "", errors.New(cur.LastError)
			}
		}
	}
}
