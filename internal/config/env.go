package config

import (
	"errors"
	"math"
	"strings"

	engineopts "github.com/phyten/todox/internal/engine/opts"
)

func FromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	var cfg Config
	var errs []error

	setString := func(target **string, key string) {
		raw := strings.TrimSpace(getenv(key))
		if raw == "" {
			return
		}
		value := raw
		*target = &value
	}
	setList := func(target **[]string, key string) {
		raw := strings.TrimSpace(getenv(key))
		if raw == "" {
			return
		}
		list := engineopts.SplitMulti([]string{raw})
		if len(list) == 0 {
			empty := make([]string, 0)
			*target = &empty
			return
		}
		copyVals := make([]string, len(list))
		copy(copyVals, list)
		*target = &copyVals
	}
	setBool := func(target **bool, key string) {
		raw := strings.TrimSpace(getenv(key))
		if raw == "" {
			return
		}
		v, err := engineopts.ParseBool(raw, key)
		if err != nil {
			errs = append(errs, err)
			return
		}
		value := v
		*target = &value
	}
	setInt := func(target **int, key string, min, max int) {
		raw := strings.TrimSpace(getenv(key))
		if raw == "" {
			return
		}
		v, err := engineopts.ParseIntInRange(raw, key, min, max)
		if err != nil {
			errs = append(errs, err)
			return
		}
		value := v
		*target = &value
	}

	setString(&cfg.Engine.Type, "TODOX_TYPE")
	setString(&cfg.Engine.Mode, "TODOX_MODE")
	setString(&cfg.Engine.Author, "TODOX_AUTHOR")
	setList(&cfg.Engine.Paths, "TODOX_PATH")
	setList(&cfg.Engine.Excludes, "TODOX_EXCLUDE")
	setList(&cfg.Engine.PathRegex, "TODOX_PATH_REGEX")
	setBool(&cfg.Engine.ExcludeTypical, "TODOX_EXCLUDE_TYPICAL")
	setString(&cfg.Engine.Output, "TODOX_OUTPUT")
	setString(&cfg.Engine.Color, "TODOX_COLOR")
	setBool(&cfg.Engine.WithComment, "TODOX_WITH_COMMENT")
	setBool(&cfg.Engine.WithMessage, "TODOX_WITH_MESSAGE")
	setInt(&cfg.Engine.TruncAll, "TODOX_TRUNCATE", 0, math.MaxInt)
	setInt(&cfg.Engine.TruncComment, "TODOX_TRUNCATE_COMMENT", 0, math.MaxInt)
	setInt(&cfg.Engine.TruncMessage, "TODOX_TRUNCATE_MESSAGE", 0, math.MaxInt)
	setBool(&cfg.Engine.IgnoreWS, "TODOX_IGNORE_WS")
	// Allow large values here and rely on NormalizeAndValidate to enforce the
	// canonical upper bound so every input path shares the same error message.
	setInt(&cfg.Engine.Jobs, "TODOX_JOBS", 0, math.MaxInt)
	setString(&cfg.Engine.Repo, "TODOX_REPO")

	setBool(&cfg.UI.WithCommitLink, "TODOX_WITH_COMMIT_LINK")
	setBool(&cfg.UI.WithPRLinks, "TODOX_WITH_PR_LINKS")
	setBool(&cfg.UI.WithAge, "TODOX_WITH_AGE")
	setString(&cfg.UI.PRState, "TODOX_PR_STATE")
	setInt(&cfg.UI.PRLimit, "TODOX_PR_LIMIT", 1, 20)
	setString(&cfg.UI.PRPrefer, "TODOX_PR_PREFER")
	setString(&cfg.UI.Fields, "TODOX_FIELDS")
	setString(&cfg.UI.Sort, "TODOX_SORT")

	if len(errs) > 0 {
		return cfg, errors.Join(errs...)
	}
	return cfg, nil
}
