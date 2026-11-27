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
	"walrus/internal/lua"
	"walrus/internal/runtime"
	"walrus/internal/steam"
)

type CronService struct {
	store      *db.Store
	fs         *dst.FS
	driver     runtime.Driver
	clusterSvc *ClusterService
	backupSvc  *BackupService
	modSvc     *ModService
	steamMgr   *steam.Manager
	webhookSvc *WebhookService

	mu      sync.Mutex
	engine  *cron.Cron
	parser  cron.Parser
	running map[int64]bool
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
		running:    make(map[int64]bool),
	}
}

func (s *CronService) SetModService(modSvc *ModService) {
	s.modSvc = modSvc
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
	s.mu.Lock()
	if s.running[taskID] {
		s.mu.Unlock()
		return
	}
	s.running[taskID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, taskID)
		s.mu.Unlock()
	}()

	t, err := s.store.GetScheduledTask(taskID)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
		summary, runErr = s.runUpdateTask(ctx, t, params, isManualRun)
	case "mod_update":
		summary, runErr = s.runModUpdateTask(ctx, t, params, isManualRun)
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
	cmd := "c_announce(" + lua.QuoteString(msg) + ")"
	sent := 0
	for _, cid := range targets {
		if st := s.driver.GetShardStatus(cid, "master"); st.State == runtime.StateRunning {
			if err := s.driver.SendCommand(cid, "master", cmd); err == nil {
				sent++
			}
		}
	}
	return fmt.Sprintf("ok (sent to %d running cluster(s))", sent), nil
}

func boolParam(params map[string]any, key string, def bool) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return def
}

func intParam(params map[string]any, key string, def int) int {
	if v, ok := params[key].(float64); ok && v >= 0 {
		return int(v)
	}
	return def
}

func formatCountdownNotice(tpl string, sec int) string {
	if strings.Contains(tpl, "%d") || strings.Contains(tpl, "%v") {
		return fmt.Sprintf(tpl, sec)
	}
	return fmt.Sprintf("%s (%ds)", tpl, sec)
}

func (s *CronService) broadcastCountdown(ctx context.Context, runningTargets []string, countdownSec int, noticeTemplate string, isManualRun bool) error {
	if countdownSec <= 0 || len(runningTargets) == 0 {
		return nil
	}
	if isManualRun && countdownSec > 10 {
		countdownSec = 5
	}
	broadcast := func(rem int) {
		cmd := "c_announce(" + lua.QuoteString(formatCountdownNotice(noticeTemplate, rem)) + ")"
		for _, cid := range runningTargets {
			if s.driver.GetShardStatus(cid, "master").State == runtime.StateRunning {
				_ = s.driver.SendCommand(cid, "master", cmd)
			}
		}
	}
	if countdownSec > 60 {
		broadcast(countdownSec)
		select {
		case <-time.After(time.Duration(countdownSec-60) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		countdownSec = 60
	}
	if countdownSec > 10 {
		broadcast(countdownSec)
		select {
		case <-time.After(time.Duration(countdownSec-10) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		countdownSec = 10
	}
	broadcast(countdownSec)
	select {
	case <-time.After(time.Duration(countdownSec) * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (s *CronService) runningClusters(clusterIDs []string) []string {
	var running []string
	for _, cid := range clusterIDs {
		if s.driver.GetShardStatus(cid, "master").State == runtime.StateRunning ||
			s.driver.GetShardStatus(cid, "caves").State == runtime.StateRunning {
			running = append(running, cid)
		}
	}
	return running
}

func (s *CronService) hasOnlinePlayers(clusterIDs []string) bool {
	for _, cid := range clusterIDs {
		if len(s.driver.GetWorldInfo(cid).OnlinePlayers) > 0 {
			return true
		}
	}
	return false
}

func (s *CronService) backupClustersPreUpdate(ctx context.Context, clusterIDs []string, note string) error {
	if s.backupSvc == nil {
		return nil
	}
	for _, cid := range clusterIDs {
		if _, err := s.backupSvc.CreateBackup(ctx, cid, "pre_update", note, "cron"); err != nil {
			if s.webhookSvc != nil {
				s.webhookSvc.Dispatch("backup.failed", "Pre-Update Backup Failed", err.Error(), cid)
			}
			return err
		}
	}
	return nil
}

func (s *CronService) restartClustersBestEffort(ctx context.Context, clusterIDs []string) {
	for _, cid := range clusterIDs {
		_ = s.clusterSvc.StartCluster(ctx, cid)
	}
}

func (s *CronService) runRestartTask(ctx context.Context, t *db.ScheduledTask, params map[string]any, isManualRun bool) (string, error) {
	targets, err := s.targetClusters(t.ClusterID)
	if err != nil {
		return "", err
	}
	countdownSec := intParam(params, "countdown_sec", 60)
	noticeTemplate, _ := params["message"].(string)
	if strings.TrimSpace(noticeTemplate) == "" {
		noticeTemplate = "Server scheduled restart in %ds!"
	}

	runningTargets := s.runningClusters(targets)
	if len(runningTargets) == 0 {
		return "skipped (no running clusters)", nil
	}

	if err := s.broadcastCountdown(ctx, runningTargets, countdownSec, noticeTemplate, isManualRun); err != nil {
		return "", err
	}

	for _, cid := range runningTargets {
		if err := s.clusterSvc.RestartCluster(ctx, cid); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("ok (restarted %d cluster(s))", len(runningTargets)), nil
}

func (s *CronService) runUpdateTask(ctx context.Context, t *db.ScheduledTask, params map[string]any, isManualRun bool) (string, error) {
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

	autoUpgrade := boolParam(params, "auto_upgrade", true)
	if !autoUpgrade {
		return fmt.Sprintf("update available (%s -> %s, notify only)", st.LocalVersion, st.RemoteVersion), nil
	}

	onlyWhenEmpty := boolParam(params, "only_when_empty", true)
	countdownSec := intParam(params, "countdown_sec", 60)
	backupBeforeUpdate := boolParam(params, "backup_before_update", false)
	noticeTemplate, _ := params["message"].(string)
	if strings.TrimSpace(noticeTemplate) == "" {
		noticeTemplate = "DST server update available, restarting in %ds!"
	}

	ids, _ := s.fs.ListClusterIDs()
	running := s.runningClusters(ids)
	hasPlayers := s.hasOnlinePlayers(running)
	if onlyWhenEmpty && hasPlayers {
		return "deferred (players online)", nil
	}

	if hasPlayers && countdownSec > 0 {
		if err := s.broadcastCountdown(ctx, running, countdownSec, noticeTemplate, isManualRun); err != nil {
			return "", err
		}
	}

	for _, cid := range running {
		if s.driver.GetShardStatus(cid, "master").State == runtime.StateRunning {
			_ = s.driver.SendCommand(cid, "master", `c_announce("Updating DST server to new build, restarting shortly...")`)
		}
		_ = s.clusterSvc.StopCluster(ctx, cid)
	}

	if backupBeforeUpdate {
		backupNote := fmt.Sprintf("Pre-update backup before DST %s (%s)", st.RemoteVersion, t.Name)
		if err := s.backupClustersPreUpdate(ctx, ids, backupNote); err != nil {
			s.restartClustersBestEffort(ctx, running)
			return "", err
		}
	}

	if err := s.steamMgr.UpdateDSTServer(branch, true); err != nil {
		s.restartClustersBestEffort(ctx, running)
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
				s.restartClustersBestEffort(ctx, running)
				if s.webhookSvc != nil {
					s.webhookSvc.Dispatch("steam.update_complete", "DST Server Updated", fmt.Sprintf("Updated to build %s and restarted %d cluster(s)", cur.LocalVersion, len(running)), "*")
				}
				return fmt.Sprintf("updated to %s", cur.LocalVersion), nil
			}
			if cur.TaskState == steam.TaskFailed {
				s.restartClustersBestEffort(ctx, running)
				return "", errors.New(cur.LastError)
			}
		}
	}
}

func formatOutdatedModSummary(outdated []OutdatedMod) string {
	names := make([]string, 0, len(outdated))
	for i, item := range outdated {
		if i >= 5 {
			names = append(names, fmt.Sprintf("+%d more", len(outdated)-i))
			break
		}
		label := strings.TrimSpace(item.Name)
		if label == "" {
			label = item.ModID
		}
		names = append(names, label)
	}
	return strings.Join(names, ", ")
}

func (s *CronService) runModUpdateTask(ctx context.Context, t *db.ScheduledTask, params map[string]any, isManualRun bool) (string, error) {
	if s.modSvc == nil {
		return "", errors.New("mod service not configured")
	}
	targets, err := s.targetClusters(t.ClusterID)
	if err != nil {
		return "", err
	}

	res, err := s.modSvc.CheckModUpdates(ctx, targets)
	if err != nil {
		return "", err
	}
	if res.CheckedCount == 0 {
		return "up to date (no enabled workshop mods)", nil
	}
	if len(res.Outdated) == 0 {
		return fmt.Sprintf("up to date (%d mod(s))", res.CheckedCount), nil
	}

	modSummary := formatOutdatedModSummary(res.Outdated)
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch(
			"mod.update_available",
			"Workshop Mod Updates Available",
			fmt.Sprintf("%d mod(s) have updates: %s", len(res.Outdated), modSummary),
			t.ClusterID,
		)
	}

	autoUpgrade := boolParam(params, "auto_upgrade", true)
	if !autoUpgrade {
		return fmt.Sprintf("updates available (%d mod(s), notify only)", len(res.Outdated)), nil
	}

	onlyWhenEmpty := boolParam(params, "only_when_empty", true)
	countdownSec := intParam(params, "countdown_sec", 60)
	backupBeforeUpdate := boolParam(params, "backup_before_update", false)
	noticeTemplate, _ := params["message"].(string)
	if strings.TrimSpace(noticeTemplate) == "" {
		noticeTemplate = "Workshop mod update available, restarting in %ds!"
	}

	running := s.runningClusters(res.AffectedClusters)
	hasPlayers := s.hasOnlinePlayers(running)
	if onlyWhenEmpty && hasPlayers {
		return fmt.Sprintf("deferred (%d mod(s) outdated, players online)", len(res.Outdated)), nil
	}

	if hasPlayers && countdownSec > 0 {
		if err := s.broadcastCountdown(ctx, running, countdownSec, noticeTemplate, isManualRun); err != nil {
			return "", err
		}
	}

	for _, cid := range running {
		if s.driver.GetShardStatus(cid, "master").State == runtime.StateRunning {
			_ = s.driver.SendCommand(cid, "master", `c_announce("Updating Workshop mods, restarting shortly...")`)
		}
		_ = s.clusterSvc.StopCluster(ctx, cid)
	}

	if backupBeforeUpdate {
		backupNote := fmt.Sprintf("Pre-update backup before updating %d mod(s) (%s)", len(res.Outdated), t.Name)
		if err := s.backupClustersPreUpdate(ctx, res.AffectedClusters, backupNote); err != nil {
			s.restartClustersBestEffort(ctx, running)
			return "", err
		}
	}

	if err := s.modSvc.UpdateWorkshopMods(ctx, res.Outdated); err != nil {
		s.restartClustersBestEffort(ctx, running)
		return "", err
	}

	s.restartClustersBestEffort(ctx, running)
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch(
			"mod.update_complete",
			"Workshop Mods Updated",
			fmt.Sprintf("Updated %d mod(s) (%s) and restarted %d cluster(s)", len(res.Outdated), modSummary, len(running)),
			t.ClusterID,
		)
	}
	return fmt.Sprintf("updated %d mod(s), restarted %d cluster(s)", len(res.Outdated), len(running)), nil
}
