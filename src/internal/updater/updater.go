package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"nomad-podman-autoupdate/internal/common"
	"nomad-podman-autoupdate/internal/nomadutil"
	"nomad-podman-autoupdate/internal/podmanutil"

	nomadApi "github.com/hashicorp/nomad/api"
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

				success, err := u.tryUpdateTask(taskLogger, job, t)
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
						slog.String("old_target", oldTarget),
						slog.String("new_target", newTarget),
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

func (u *Updater) tryUpdateTask(taskLogger *slog.Logger, job *nomadApi.Job, task *nomadApi.Task) (bool, error) {
	var (
		taskImage     string
		taskImageRoot string
		imageRef      strings.Builder
	)
	if v, ok := task.Config["image"]; !ok {
		return false, errors.New("invalid task: task 'image' config property is not set")
	} else {
		if taskImage, ok = v.(string); !ok {
			return false, errors.New("invalid task: task 'image' config property is not a string")
		}
	}

	if pos := strings.IndexAny(taskImage, "$:@"); pos > 0 {
		taskImageRoot = taskImage[0:pos]
	} else {
		taskImageRoot = taskImage
	}

	origSrc, sourceIsSet := task.Meta[common.UpdateableTaskMetaSource]
	origTgt, ok := task.Meta[common.UpdateableTaskMetaTarget]
	if !ok {
		return false, errors.New("expected task meta property '" + common.UpdateableTaskMetaTarget + "' is not set")
	}

	if !sourceIsSet && len(origTgt) > 0 && origTgt[0] == '@' {
		return false, fmt.Errorf("task reference '%s' specified in '"+common.UpdateableTaskMetaTarget+"' is not an updatable tag format", origTgt)
	}

	var pullTag string
	if sourceIsSet {
		pullTag = origSrc
	} else {
		pullTag = origTgt
	}

	imageRef.WriteString(taskImageRoot)
	if len(pullTag) > 0 {
		if pullTag[0] != ':' && pullTag[0] != '@' {
			imageRef.WriteByte(':')
		}
		imageRef.WriteString(pullTag)
	}

	_, err := podmanutil.PullImage(u.PodmanConn, imageRef.String())
	if err != nil {
		return false, fmt.Errorf("failed to pull image: %w", err)
	}
	imgInfo, err := podmanutil.ImageInfo(u.PodmanConn, imageRef.String())
	if err != nil {
		return false, fmt.Errorf("failed to inspect image: %w", err)
	}

	newTgt := "@" + imgInfo.Digest.String()

	if newTgt == origTgt {
		taskLogger.Debug("pulled image digest matches task digest in '"+common.UpdateableTaskMetaTarget+"'", slog.String("image", imageRef.String()), slog.String("digest", imgInfo.Digest.String()))
		return false, nil
	}
	taskLogger.Debug("new digest found for task image", slog.String("image", imageRef.String()), slog.String("digest", imgInfo.Digest.String()))

	task.Meta[common.UpdateableTaskMetaTarget] = newTgt
	taskLogger.Debug("updated meta property '"+common.UpdateableTaskMetaTarget+"' for task", slog.String("old_value", origTgt), slog.String("value", newTgt))
	if !sourceIsSet {
		taskLogger.Debug("adding meta property '"+common.UpdateableTaskMetaSource+"' for task", slog.String("value", origTgt))
		task.Meta[common.UpdateableTaskMetaSource] = origTgt
	}

	return true, nil
}
