package updater

import (
	"fmt"
	"log/slog"

	"nomad-podman-autoupdate/internal/common"
	"nomad-podman-autoupdate/internal/nomadutil"

	nomadApi "github.com/hashicorp/nomad/api"
)

type Updater struct {
	NomadClient *nomadApi.Client
}

func (u *Updater) TryUpdateJob(jobId string) error {
	job, err := nomadutil.GetJobInfo(u.NomadClient, jobId)
	if err != nil {
		return fmt.Errorf("failed to load job '%s': %w", jobId, err)
	}

	var (
		taskFound   = false
		taskUpdated = false
	)
	for _, g := range job.TaskGroups {
		for _, t := range g.Tasks {
			if oldTarget, ok := t.Meta[common.UpdateableTaskMetaTarget]; ok {
				taskFound = true

				success, err := u.tryUpdateTask(job, t)
				if err != nil {
					return fmt.Errorf("failed to update task '%s' in group '%s' for job '%s': %w", t.Name, *g.Name, jobId, err)
				}

				if success {
					taskUpdated = true

					newTarget, ok := t.Meta[common.UpdateableTaskMetaTarget]
					if !ok {
						return fmt.Errorf("expected meta property '%s' has been unset", common.UpdateableTaskMetaTarget)
					}

					slog.Info("task update found",
						slog.String("job", jobId),
						slog.String("group", *g.Name),
						slog.String("task", t.Name),
						slog.String("old_target", oldTarget),
						slog.String("new_target", newTarget),
					)
				} else {
					slog.Debug("no update found for task",
						slog.String("job", jobId),
						slog.String("group", *g.Name),
						slog.String("task", t.Name),
					)
				}
			}
		}
	}
	if !taskFound {
		return fmt.Errorf("no updatable tasks found for job '%s'", jobId)
	}

	if !taskUpdated {
		slog.Info("no task updates found for job", slog.String("job", jobId))
		return nil
	}

	jobVer := int(*job.Version)
	jobSrc, err := nomadutil.GetJobSource(u.NomadClient, jobId, &jobVer)

	if jobSrc == nil {
		slog.Info("no source found for current job version", slog.String("job", jobId), slog.Int("version", jobVer))
	}

	return nil
}

func (u *Updater) tryUpdateTask(job *nomadApi.Job, task *nomadApi.Task) (bool, error) {
	return false, nil
}
