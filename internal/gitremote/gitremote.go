package gitremote

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/phyten/todox/internal/execx"
)

// Info は Git リモートから抽出したホスト・オーナー・リポジトリ情報です。
type Info struct {
	Host   string
	Owner  string
	Repo   string
	Scheme string
}

// Detect は repoDir の Git リモート (origin) を解析して Info を返します。
func Detect(ctx context.Context, runner execx.Runner, repoDir string) (Info, error) {
	if runner == nil {
		runner = execx.DefaultRunner()
	}
	remoteName := strings.TrimSpace(os.Getenv("TODOX_LINK_REMOTE"))
	if remoteName == "" {
		remoteName = "origin"
	}
	key := fmt.Sprintf("remote.%s.url", remoteName)
	stdout, stderr, err := runner.Run(ctx, repoDir, "git", "config", "--get", key)
	if err != nil {
		if len(stderr) > 0 {
			return Info{}, fmt.Errorf("git config failed for %s: %w: %s", key, err, strings.TrimSpace(string(stderr)))
		}
		return Info{}, fmt.Errorf("git config failed for %s: %w", key, err)
	}
	remote := strings.TrimSpace(string(stdout))
	if remote == "" {
		return Info{}, fmt.Errorf("%s is empty", key)
	}
	info, err := Parse(remote)
	if err != nil {
		return Info{}, err
	}
	if override := normalizeSchemeOverride(os.Getenv("TODOX_LINK_SCHEME")); override != "" {
		info.Scheme = override
	}
	return info, nil
}

// Parse は remote.origin.url の値を解析し、Info を返します。
func Parse(raw string) (Info, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Info{}, errors.New("empty remote url")
	}
	if strings.HasPrefix(raw, "git@") {
		// git@github.com:owner/repo.git
		withoutUser := strings.TrimPrefix(raw, "git@")
		parts := strings.SplitN(withoutUser, ":", 2)
		if len(parts) != 2 {
			return Info{}, fmt.Errorf("invalid ssh remote: %s", raw)
		}
		host := strings.ToLower(strings.TrimSpace(parts[0]))
		owner, repo, err := splitPath(parts[1])
		if err != nil {
			return Info{}, err
		}
		return Info{Host: host, Owner: owner, Repo: repo}, nil
	}
	if strings.HasPrefix(raw, "ssh://") || strings.HasPrefix(raw, "git://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Info{}, fmt.Errorf("invalid remote url: %w", err)
		}
		cleaned, err := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
		if err != nil {
			return Info{}, fmt.Errorf("invalid remote path: %w", err)
		}
		owner, repo, err := splitPath(cleaned)
		if err != nil {
			return Info{}, err
		}
		host := strings.ToLower(strings.TrimSpace(u.Host))
		return Info{Host: host, Owner: owner, Repo: repo}, nil
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Info{}, fmt.Errorf("invalid remote url: %w", err)
		}
		cleaned, err := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
		if err != nil {
			return Info{}, fmt.Errorf("invalid remote path: %w", err)
		}
		owner, repo, err := splitPath(cleaned)
		if err != nil {
			return Info{}, err
		}
		host := strings.ToLower(strings.TrimSpace(u.Host))
		scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
		return Info{Host: host, Owner: owner, Repo: repo, Scheme: scheme}, nil
	}
	return Info{}, fmt.Errorf("unsupported remote url: %s", raw)
}

func splitPath(p string) (string, string, error) {
	cleaned := strings.TrimSpace(p)
	cleaned = strings.TrimSuffix(cleaned, ".git")
	cleaned = strings.Trim(cleaned, "/\\")
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	cleaned = filepath.ToSlash(cleaned)
	if cleaned == "" {
		return "", "", errors.New("missing owner/repo in remote url")
	}
	segments := strings.Split(cleaned, "/")
	if len(segments) < 2 {
		return "", "", errors.New("remote url must include owner and repo")
	}
	owner := segments[len(segments)-2]
	repo := segments[len(segments)-1]
	if owner == "" || repo == "" {
		return "", "", errors.New("invalid owner or repo in remote url")
	}
	return owner, repo, nil
}

// WebURL はリポジトリのブラウズ用ベース URL を返します。
func (i Info) WebURL() string {
	host := strings.TrimSuffix(i.Host, "/")
	return fmt.Sprintf("%s://%s/%s/%s", i.NormalizedScheme(), host, url.PathEscape(i.Owner), url.PathEscape(i.Repo))
}

// APIBaseURL は REST API ベース URL を返します (GitHub.com 以外は /api/v3)。
func (i Info) APIBaseURL() string {
	host := strings.TrimSuffix(i.Host, "/")
	if strings.EqualFold(host, "github.com") {
		return "https://api.github.com"
	}
	return fmt.Sprintf("%s://%s/api/v3", i.NormalizedScheme(), host)
}

// BlobPath は URL で利用するパスを返します。
func BlobPath(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for idx, part := range parts {
		parts[idx] = url.PathEscape(part)
	}
	return path.Join(parts...)
}

// NormalizedScheme はリンク生成に利用するスキームを返します。
// 現状は http/https のみを許可し、空の場合は https を既定とします。
func (i Info) NormalizedScheme() string {
	if override := normalizeSchemeOverride(os.Getenv("TODOX_LINK_SCHEME")); override != "" {
		return override
	}
	scheme := strings.ToLower(strings.TrimSpace(i.Scheme))
	if scheme == "http" {
		return "http"
	}
	return "https"
}

func normalizeSchemeOverride(raw string) string {
	scheme := strings.ToLower(strings.TrimSpace(raw))
	switch scheme {
	case "http", "https":
		return scheme
	case "":
		return ""
	default:
		return ""
	}
}
