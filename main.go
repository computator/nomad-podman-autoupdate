package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/computator/nomad-podman-autoupdate/internal/common"
	"github.com/computator/nomad-podman-autoupdate/internal/nomadutil"
	"github.com/computator/nomad-podman-autoupdate/internal/podmanutil"
	"github.com/computator/nomad-podman-autoupdate/internal/updater"

	nomadApi "github.com/hashicorp/nomad/api"
)

func jobs() bool {
	nclient, err := nomadApi.NewClient(nomadApi.DefaultConfig())
	if err != nil {
		slog.Error("failed to create nomad client", slog.Any("err", err))
		return false
	}
	slog.Log(context.Background(), common.LevelTrace, "created nomad client", slog.Any("client", nclient))

	updater, err := updater.NewUpdater(nclient, podmanutil.NewDefaultConnection)
	if err != nil {
		slog.Error("error initializing updater", slog.Any("err", err))
		return false
	}
	defer updater.PodmanConnPool.Close()

	jobs, err := nomadutil.GetUpdateableJobs(nclient, false)
	if err != nil {
		slog.Error("failed to get updateable jobs", slog.Any("err", err))
		return false
	}
	slog.Debug("found updatable jobs", slog.Int("count", len(jobs)), slog.Any("ids", jobs))
	if len(jobs) == 0 {
		slog.Info("no updatable jobs found")
	}

	var (
		updateErrors = false
		wg           sync.WaitGroup
	)
	for _, jobId := range jobs {
		wg.Go(func() {
			if err := updater.TryUpdateJob(jobId); err != nil {
				if errors.Is(err, nomadutil.ErrModifyIndexConflict) {
					slog.Warn("task updates found but not applied because the job has been modified elsewhere", slog.String("id", jobId))
				} else {
					slog.Error("failed to update job", slog.String("id", jobId), slog.Any("err", err))
					updateErrors = true
					return
				}
			}
		})
	}
	wg.Wait()

	if updateErrors {
		return false
	}

	return true
}

func main() {
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: common.LevelTrace}),
	))

	if ok := jobs(); !ok {
		os.Exit(1)
	}
}
