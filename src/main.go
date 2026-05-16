package main

import (
	"fmt"
	"log/slog"
	"os"

	"nomad-podman-autoupdate/internal/nomadutil"

	nomadApi "github.com/hashicorp/nomad/api"
)

func jobs() bool {
	nclient, err := nomadApi.NewClient(nomadApi.DefaultConfig())
	if err != nil {
		slog.Error("failed to create nomad client", slog.Any("err", err))
		return false
	}
	slog.Debug("created nomad client", slog.Any("client", nclient))

	jobs, err := nomadutil.GetUpdateableJobs(nclient)
	if err != nil {
		slog.Error("failed to get updateable jobs", slog.Any("err", err))
		return false
	}
	slog.Debug("found updatable jobs", slog.Any("ids", jobs))

	for _, jobId := range jobs {
		jobSrc, err := nomadutil.GetJobSource(nclient, jobId, nil)
		if err != nil {
			slog.Error("failed to get nomad job source definition", slog.String("id", jobId), slog.Any("err", err))
			return false
		}
		slog.Info("got job source definition",
			slog.String("id", jobId),
			slog.String("format", jobSrc.Format),
			// slog.String("source", jobSrc.Source),
			slog.Any("variable_flags", jobSrc.VariableFlags),
			slog.String("variables", jobSrc.Variables),
		)
		fmt.Println(jobSrc.Source)
	}

	return true
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if ok := jobs(); !ok {
		os.Exit(1)
	}
}
