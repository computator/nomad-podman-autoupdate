package main

import (
	"errors"
	"log/slog"
	"os"
	"sync"

	"nomad-podman-autoupdate/internal/nomadutil"
	"nomad-podman-autoupdate/internal/podmanutil"
	"nomad-podman-autoupdate/internal/updater"

	nomadApi "github.com/hashicorp/nomad/api"
)

func jobs() bool {
	nclient, err := nomadApi.NewClient(nomadApi.DefaultConfig())
	if err != nil {
		slog.Error("failed to create nomad client", slog.Any("err", err))
		return false
	}
	slog.Debug("created nomad client", slog.Any("client", nclient))

	pconn, err := podmanutil.NewDefaultConnection()
	if err != nil {
		slog.Error("failed to create connection to podman", slog.Any("err", err))
		return false
	}
	slog.Debug("created podman connection", slog.Any("connection", pconn))

	updater := updater.NewUpdater(nclient, pconn)

	jobs, err := nomadutil.GetUpdateableJobs(nclient)
	if err != nil {
		slog.Error("failed to get updateable jobs", slog.Any("err", err))
		return false
	}
	slog.Debug("found updatable jobs", slog.Any("ids", jobs))

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
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if ok := jobs(); !ok {
		os.Exit(1)
	}
}
