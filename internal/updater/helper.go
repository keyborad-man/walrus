package updater

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const upgradeSpecEnvKey = "WALRUS_UPGRADE_SPEC"

// UpgradeSpec is passed via environment variable to the temporary helper container
// that performs the stop -> rename -> create -> start -> cleanup swap.
type UpgradeSpec struct {
	OldContainerID string         `json:"old_container_id"`
	ContainerName  string         `json:"container_name"`
	TargetImage    string         `json:"target_image"`
	DockerSock     string         `json:"docker_sock"`
	CreatePayload  map[string]any `json:"create_payload"`
}

func encodeUpgradeSpec(spec UpgradeSpec) (string, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func decodeUpgradeSpec(encoded string) (UpgradeSpec, error) {
	var spec UpgradeSpec
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return spec, fmt.Errorf("decode upgrade spec base64: %w", err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("decode upgrade spec json: %w", err)
	}
	return spec, nil
}

// RunUpgradeHelper executes the container replacement procedure when Walrus is invoked with
// `--self-upgrade-helper`. It stops the old container gracefully (giving Walrus time to save
// running DST shards), creates and starts the replacement container with the new image, and
// automatically rolls back to the old container if anything fails.
func RunUpgradeHelper() error {
	encoded := os.Getenv(upgradeSpecEnvKey)
	if strings.TrimSpace(encoded) == "" {
		return errors.New("missing WALRUS_UPGRADE_SPEC environment variable")
	}
	spec, err := decodeUpgradeSpec(encoded)
	if err != nil {
		return err
	}
	client := NewDockerClient(spec.DockerSock)
	return ExecuteContainerSwap(context.Background(), client, spec, 1200*time.Millisecond)
}

// ExecuteContainerSwap performs the actual Docker API swap sequence. Extracted for unit testing.
func ExecuteContainerSwap(ctx context.Context, client *DockerClient, spec UpgradeSpec, initialDelay time.Duration) error {
	if spec.OldContainerID == "" || spec.ContainerName == "" || spec.TargetImage == "" {
		return errors.New("incomplete upgrade spec")
	}
	if initialDelay > 0 {
		select {
		case <-time.After(initialDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	slog.Info("upgrade helper: stopping current walrus container gracefully",
		"old_id", spec.OldContainerID,
		"name", spec.ContainerName,
		"target_image", spec.TargetImage,
	)

	stopCtx, cancelStop := context.WithTimeout(ctx, 45*time.Second)
	defer cancelStop()
	if err := client.StopContainer(stopCtx, spec.OldContainerID, 35); err != nil {
		return fmt.Errorf("stop old container: %w", err)
	}

	backupName := fmt.Sprintf("%s-walrus-old-%d", strings.TrimPrefix(spec.ContainerName, "/"), time.Now().Unix())
	renameCtx, cancelRename := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRename()
	if err := client.RenameContainer(renameCtx, spec.OldContainerID, backupName); err != nil {
		// Try restarting the old container so we don't leave the panel down.
		_ = client.StartContainer(context.Background(), spec.OldContainerID)
		return fmt.Errorf("rename old container: %w", err)
	}

	rollback := func(newID string, cause error) error {
		slog.Error("upgrade helper: rolling back to previous container", "err", cause)
		rbCtx, cancelRB := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelRB()
		if newID != "" {
			_ = client.RemoveContainer(rbCtx, newID, true)
		}
		_ = client.RenameContainer(rbCtx, spec.OldContainerID, spec.ContainerName)
		_ = client.StartContainer(rbCtx, spec.OldContainerID)
		return cause
	}

	createCtx, cancelCreate := context.WithTimeout(ctx, 30*time.Second)
	defer cancelCreate()
	newID, err := client.CreateContainer(createCtx, spec.ContainerName, spec.CreatePayload)
	if err != nil {
		return rollback("", fmt.Errorf("create new container: %w", err))
	}

	startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
	defer cancelStart()
	if err := client.StartContainer(startCtx, newID); err != nil {
		return rollback(newID, fmt.Errorf("start new container: %w", err))
	}

	slog.Info("upgrade helper: new walrus container started, removing old container",
		"new_id", newID,
		"old_id", spec.OldContainerID,
	)
	rmCtx, cancelRM := context.WithTimeout(ctx, 20*time.Second)
	defer cancelRM()
	if err := client.RemoveContainer(rmCtx, spec.OldContainerID, true); err != nil {
		slog.Warn("upgrade helper: failed to remove old container (new container is running)", "err", err)
	}
	return nil
}
