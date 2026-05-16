package nomadutil

import (
	"errors"
	"fmt"
	"log/slog"

	"nomad-podman-autoupdate/internal/common"

	nomadApi "github.com/hashicorp/nomad/api"
)

func GetUpdateableJobs(nclient *nomadApi.Client) ([]string, error) {
	jobs, _, err := nclient.Jobs().List(&nomadApi.QueryOptions{Filter: common.UpdateableJobsFilterExpr})
	if err != nil {
		return []string{}, fmt.Errorf("failed to list nomad jobs: %w", err)
	}
	slog.Debug("loaded nomad jobs", slog.Int("count", len(jobs)))
	jobIds := make([]string, 0, len(jobs))
	for _, j := range jobs {
		slog.Debug("got job listing", slog.Any("job", j))
		jobIds = append(jobIds, j.ID)
	}
	slog.Debug("job id list", slog.Any("ids", jobIds))
	return jobIds, nil
}

func GetJobInfo(nclient *nomadApi.Client, jobId string) (*nomadApi.Job, error) {
	job, _, err := nclient.Jobs().Info(jobId, &nomadApi.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get info for nomad job '%s': %w", jobId, err)
	}
	slog.Debug("got job info", slog.String("id", jobId), slog.Any("job", job))

	return job, nil
}

func GetJobSource(nclient *nomadApi.Client, jobId string, jobVersion *int) (*nomadApi.JobSubmission, error) {
	if jobVersion == nil {
		slog.Debug("no job source version specified, loading from job info")
		job, err := GetJobInfo(nclient, jobId)
		if err != nil {
			return nil, err
		}
		jobVersion = new(int)
		*jobVersion = int(*job.Version)
	}

	slog.Debug("loading job source", slog.Int("version", *jobVersion))
	jobSrc, _, err := nclient.Jobs().Submission(jobId, *jobVersion, &nomadApi.QueryOptions{})
	if err != nil {
		respErr := nomadApi.UnexpectedResponseError{}
		if errors.As(err, &respErr) && respErr.StatusCode() == 404 {
			jobSrc = nil
		} else {
			return nil, fmt.Errorf("failed to get source for nomad job '%s': %w", jobId, err)
		}
	}
	if jobSrc == nil {
		slog.Debug("no job source found", slog.String("id", jobId), slog.Int("version", *jobVersion))
	} else {
		slog.Debug("got job source", slog.String("id", jobId), slog.Any("source", jobSrc))
	}

	return jobSrc, nil
}
