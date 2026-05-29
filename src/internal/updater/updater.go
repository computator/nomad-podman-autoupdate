package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	nomadApi "github.com/hashicorp/nomad/api"

	"nomad-podman-autoupdate/internal/common"
	"nomad-podman-autoupdate/internal/nomadutil"
	"nomad-podman-autoupdate/internal/podmanutil"
)

type Updater struct {
	NomadClient *nomadApi.Client
	PodmanConn  context.Context
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

				taskLogger := slog.With(
					slog.String("job", jobId),
					slog.String("group", *g.Name),
					slog.String("task", t.Name),
				)

				success, err := (&taskUpdater{
					Updater: u,
					logger:  taskLogger,
					job:     job,
					task:    t,
				}).tryUpdateTask()
				if err != nil {
					return fmt.Errorf("failed to update task '%s' in group '%s' for job '%s': %w", t.Name, *g.Name, jobId, err)
				}

				if success {
					taskUpdated = true

					newTarget, ok := t.Meta[common.UpdateableTaskMetaTarget]
					if !ok {
						return errors.New("expected meta property '" + common.UpdateableTaskMetaTarget + "' has been unset")
					}

					taskLogger.Info("task update found",
						slog.String("target", newTarget),
						slog.String("old_target", oldTarget),
					)
				} else {
					taskLogger.Debug("no update found for task")
				}
			}
		}
	}
	if !taskFound {
		return fmt.Errorf("no updatable tasks found in job '%s'", jobId)
	}

	if !taskUpdated {
		slog.Info("no task updates found for job", slog.String("job", jobId))
		return nil
	}

	oldIndex := new(int(*job.JobModifyIndex))
	newIndex, err := nomadutil.UpsertJob(u.NomadClient, job, oldIndex)
	if err != nil {
		if errors.Is(err, nomadutil.ErrModifyIndexConflict) {
			return fmt.Errorf("job modified elsewhere during update process '%s': %w", jobId, err)
		} else {
			return fmt.Errorf("failed to submit modified job with updated tasks '%s': %w", jobId, err)
		}
	}
	slog.Info("submitted modified job with updated tasks", slog.String("job", jobId), slog.Int("job_index", newIndex))

	return nil
}

type taskUpdater struct {
	logger *slog.Logger
	job    *nomadApi.Job
	task   *nomadApi.Task
	*Updater
}

func (tu *taskUpdater) tryUpdateTask() (bool, error) {
	imgPullRef, err := tu.getImagePullRef()
	if err != nil {
		return false, fmt.Errorf("failed to get current task image reference: %w", err)
	}

	_, err = podmanutil.PullImage(tu.PodmanConn, imgPullRef)
	if err != nil {
		return false, fmt.Errorf("failed to pull image: %w", err)
	}
	imgInfo, err := podmanutil.ImageInfo(tu.PodmanConn, imgPullRef)
	if err != nil {
		return false, fmt.Errorf("failed to inspect image: %w", err)
	}

	currDigestTag, err := tu.getRefDigest()
	if err != nil {
		return false, fmt.Errorf("failed to get current task image digest reference: %w", err)
	}

	newRefTag := "@" + imgInfo.Digest.String()
	if newRefTag == currDigestTag {
		tu.logger.Debug(
			"pulled image digest matches current task digest reference",
			slog.String("image", imgPullRef),
			slog.String("digest", imgInfo.Digest.String()),
		)
		return false, nil
	}

	tu.logger.Debug(
		"new digest found for task image",
		slog.String("image", imgPullRef),
		slog.String("digest", imgInfo.Digest.String()),
	)

	if err := tu.setImageRefData(newRefTag); err != nil {
		return false, fmt.Errorf("failed to set new task image reference data: %w", err)
	}

	return true, nil
}
