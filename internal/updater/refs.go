package updater

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"nomad-podman-autoupdate/internal/common"
)

func (tu *taskUpdater) getRefDigest() (string, error) {
	tgt, ok := tu.task.Meta[common.UpdateableTaskMetaTarget]
	if !ok {
		return "", errors.New("expected task meta property '" + common.UpdateableTaskMetaTarget + "' is not set")
	}

	if len(tgt) > 0 && tgt[0] == '@' {
		return tgt, nil
	}

	return "", nil
}

func (tu *taskUpdater) getImagePullRef() (string, error) {
	var imageRef strings.Builder

	if v, ok := tu.task.Config["image"]; !ok {
		return "", errors.New("invalid task: task 'image' config property is not set")
	} else {
		if img, ok := v.(string); !ok {
			return "", errors.New("invalid task: task 'image' config property is not a string")
		} else {
			if pos := strings.IndexAny(img, "$:@"); pos > 0 {
				imageRef.WriteString(img[0:pos])
			} else {
				imageRef.WriteString(img)
			}
		}
	}

	var pullTag string
	if src, ok := tu.task.Meta[common.UpdateableTaskMetaSource]; ok {
		pullTag = src
	} else {
		if tgt, ok := tu.task.Meta[common.UpdateableTaskMetaTarget]; !ok {
			return "", errors.New("expected task meta property '" + common.UpdateableTaskMetaTarget + "' is not set")
		} else if len(tgt) > 0 && tgt[0] == '@' {
			return "", fmt.Errorf("task reference '%s' specified in '"+common.UpdateableTaskMetaTarget+"' is not an updatable tag format", tgt)
		} else {
			pullTag = tgt
		}
	}

	if len(pullTag) > 0 {
		if pullTag[0] != ':' && pullTag[0] != '@' {
			imageRef.WriteByte(':')
		}
		imageRef.WriteString(pullTag)
	}

	return imageRef.String(), nil
}

func (tu *taskUpdater) setImageRefData(targetRefTag string) error {
	origTgt, ok := tu.task.Meta[common.UpdateableTaskMetaTarget]
	if !ok {
		return errors.New("expected task meta property '" + common.UpdateableTaskMetaTarget + "' is not set")
	}

	tu.task.Meta[common.UpdateableTaskMetaTarget] = targetRefTag
	tu.logger.Debug(
		"updated meta property '"+common.UpdateableTaskMetaTarget+"' for task",
		slog.String("value", targetRefTag),
		slog.String("old_value", origTgt),
	)

	if _, ok := tu.task.Meta[common.UpdateableTaskMetaSource]; !ok {
		tu.logger.Debug(
			"adding meta property '"+common.UpdateableTaskMetaSource+"' for task",
			slog.String("value", origTgt),
		)
		tu.task.Meta[common.UpdateableTaskMetaSource] = origTgt
	}

	return nil
}
