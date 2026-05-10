package main

import (
	"log/slog"
	"nomad-podman-autoupdate/internal/updater"
	"os"

	nomadApi "github.com/hashicorp/nomad/api"
)

func jobs() bool {
	var upd *updater.Updater

	if client, err := nomadApi.NewClient(nomadApi.DefaultConfig()); err != nil {
		slog.Error("failed to create nomad client", slog.Any("err", err))
		return false
	} else {
		slog.Debug("created nomad client", slog.Any("client", client))
		upd = updater.NewUpdater(client)
	}

	jobs, err := upd.GetUpdateableJobs()
	if err != nil {
		slog.Error("failed to get updateable jobs", slog.Any("err", err))
		return false
	}
	slog.Debug("found updatable jobs", slog.Any("ids", jobs))

	for _, jobId := range jobs {
		jobSpec, err := upd.GetJobSpec(jobId)
		if err != nil {
			slog.Error("failed to get nomad job spec", slog.String("id", jobId), slog.Any("err", err))
			return false
		}
		slog.Info("got job spec", slog.String("id", jobId), slog.Any("source", jobSpec))
	}

	return true
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if ok := jobs(); !ok {
		os.Exit(1)
	}
}
