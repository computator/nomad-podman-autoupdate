package main

import (
	"log/slog"
	"os"

	"nomad-podman-autoupdate/internal/nomadutil"
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

	updater := &updater.Updater{NomadClient: nclient}

	jobs, err := nomadutil.GetUpdateableJobs(nclient)
	if err != nil {
		slog.Error("failed to get updateable jobs", slog.Any("err", err))
		return false
	}
	slog.Debug("found updatable jobs", slog.Any("ids", jobs))

	for _, jobId := range jobs {
		if err := updater.TryUpdateJob(jobId); err != nil {
			slog.Error("failed to update job", slog.String("id", jobId), slog.Any("err", err))
			return false
		}
	}

	return true
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if ok := jobs(); !ok {
		os.Exit(1)
	}
}
