package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/phyten/todox/internal/execx"
	"github.com/phyten/todox/internal/gitremote"
)

// PRInfo はプルリクエストの基本情報を表します。
type PRInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
}

// Client は GitHub (Enterprise を含む) 向けの最小ラッパーです。
type Client struct {
	info       gitremote.Info
	repoDir    string
	runner     execx.Runner
	httpClient *http.Client
	token      string
}

// NewClient は GitHub クライアントを返します。
func NewClient(info gitremote.Info, repoDir string, runner execx.Runner) *Client {
	if runner == nil {
		runner = execx.DefaultRunner()
	}
	token := strings.TrimSpace(os.Getenv("GH_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	return &Client{
		info:       info,
		repoDir:    repoDir,
		runner:     runner,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
	}
}

// Host はホスト名を返します。
func (c *Client) Host() string { return c.info.Host }

// Owner はリポジトリ所有者を返します。
func (c *Client) Owner() string { return c.info.Owner }

// Repo はリポジトリ名を返します。
func (c *Client) Repo() string { return c.info.Repo }

// FindPullRequestsByCommit はコミットに紐づく PR を取得します。
func (c *Client) FindPullRequestsByCommit(ctx context.Context, sha string) ([]PRInfo, error) {
	if sha == "" {
		return nil, errors.New("commit sha is required")
	}
	data, err := c.callGH(ctx, "api", fmt.Sprintf("repos/%s/%s/commits/%s/pulls", c.info.Owner, c.info.Repo, sha))
	if err != nil {
		data, err = c.callREST(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/commits/%s/pulls", c.info.Owner, c.info.Repo, sha), nil)
		if err != nil {
			return nil, err
		}
	}
	var raw []struct {
		Number   int       `json:"number"`
		Title    string    `json:"title"`
		State    string    `json:"state"`
		HTMLURL  string    `json:"html_url"`
		MergedAt time.Time `json:"merged_at"`
	}
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	infos := make([]PRInfo, 0, len(raw))
	for _, pr := range raw {
		state := pr.State
		if strings.EqualFold(state, "closed") && !pr.MergedAt.IsZero() {
			state = "merged"
		}
		infos = append(infos, PRInfo{
			Number: pr.Number,
			Title:  pr.Title,
			State:  state,
			URL:    pr.HTMLURL,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Number < infos[j].Number })
	return infos, nil
}

// DefaultBranch は既定ブランチ名を返します。
func (c *Client) DefaultBranch(ctx context.Context) (string, error) {
	data, err := c.callGH(ctx, "api", fmt.Sprintf("repos/%s/%s", c.info.Owner, c.info.Repo))
	if err != nil {
		data, err = c.callREST(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", c.info.Owner, c.info.Repo), nil)
		if err != nil {
			return "", err
		}
	}
	var raw struct {
		DefaultBranch string `json:"default_branch"`
	}
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return "", unmarshalErr
	}
	if raw.DefaultBranch == "" {
		return "", errors.New("default branch not found")
	}
	return raw.DefaultBranch, nil
}

// BranchesWhereHead はコミットが HEAD のブランチを返します。
func (c *Client) BranchesWhereHead(ctx context.Context, sha string) ([]string, error) {
	data, err := c.callGH(ctx, "api", fmt.Sprintf("repos/%s/%s/commits/%s/branches-where-head", c.info.Owner, c.info.Repo, sha))
	if err != nil {
		data, err = c.callREST(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/commits/%s/branches-where-head", c.info.Owner, c.info.Repo, sha), nil)
		if err != nil {
			return nil, err
		}
	}
	var raw []struct {
		Name string `json:"name"`
	}
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	names := make([]string, 0, len(raw))
	for _, b := range raw {
		if b.Name != "" {
			names = append(names, b.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// FindPullRequestsByHead は head ブランチに紐づく PR を取得します (state=all)。
func (c *Client) FindPullRequestsByHead(ctx context.Context, branch string) ([]PRInfo, error) {
	if branch == "" {
		return nil, errors.New("branch is required")
	}
	args := []string{"pr", "list", "--state", "all", "--json", "number,title,state,url,mergedAt", "--head", branch}
	args = append(args, "--repo", fmt.Sprintf("%s/%s", c.info.Owner, c.info.Repo))
	if c.info.Host != "" && !strings.EqualFold(c.info.Host, "github.com") {
		args = append(args, "--hostname", c.info.Host)
	}
	out, stderr, err := c.runner.Run(ctx, c.repoDir, "gh", args...)
	if err == nil {
		var raw []struct {
			Number   int       `json:"number"`
			Title    string    `json:"title"`
			State    string    `json:"state"`
			URL      string    `json:"url"`
			MergedAt time.Time `json:"mergedAt"`
		}
		if unmarshalErr := json.Unmarshal(out, &raw); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		prs := make([]PRInfo, 0, len(raw))
		for _, pr := range raw {
			state := pr.State
			if strings.EqualFold(state, "closed") && !pr.MergedAt.IsZero() {
				state = "merged"
			}
			prs = append(prs, PRInfo{Number: pr.Number, Title: pr.Title, State: state, URL: pr.URL})
		}
		sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
		return prs, nil
	}
	if execx.IsNotFound(err) {
		return nil, err
	}
	if len(stderr) > 0 && c.token == "" {
		return nil, fmt.Errorf("gh pr list failed: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	query := url.Values{}
	query.Set("state", "all")
	query.Set("head", fmt.Sprintf("%s:%s", c.info.Owner, branch))
	data, restErr := c.callREST(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls", c.info.Owner, c.info.Repo), query)
	if restErr != nil {
		if len(stderr) > 0 {
			return nil, fmt.Errorf("gh pr list failed: %w: %s", err, strings.TrimSpace(string(stderr)))
		}
		return nil, restErr
	}
	var raw []struct {
		Number   int       `json:"number"`
		Title    string    `json:"title"`
		State    string    `json:"state"`
		HTMLURL  string    `json:"html_url"`
		MergedAt time.Time `json:"merged_at"`
	}
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	prs := make([]PRInfo, 0, len(raw))
	for _, pr := range raw {
		state := pr.State
		if strings.EqualFold(state, "closed") && !pr.MergedAt.IsZero() {
			state = "merged"
		}
		prs = append(prs, PRInfo{Number: pr.Number, Title: pr.Title, State: state, URL: pr.HTMLURL})
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
	return prs, nil
}

// CreatePullRequest は gh CLI を利用して PR を作成します。
func (c *Client) CreatePullRequest(ctx context.Context, source, base string, args []string) (string, error) {
	ghArgs := []string{"pr", "create", "-H", source, "-B", base}
	if repo := strings.TrimSpace(fmt.Sprintf("%s/%s", c.info.Owner, c.info.Repo)); repo != "/" {
		ghArgs = append(ghArgs, "--repo", repo)
	}
	if c.info.Host != "" && !strings.EqualFold(c.info.Host, "github.com") {
		ghArgs = append(ghArgs, "--hostname", c.info.Host)
	}
	ghArgs = append(ghArgs, args...)
	out, stderr, err := c.runner.Run(ctx, c.repoDir, "gh", ghArgs...)
	if err != nil {
		if len(stderr) > 0 {
			return "", fmt.Errorf("gh pr create failed: %w: %s", err, strings.TrimSpace(string(stderr)))
		}
		if execx.IsNotFound(err) {
			if strings.TrimSpace(c.token) != "" {
				return "", fmt.Errorf("gh command not found: REST helpers work via GH_TOKEN, but creating pull requests still requires the GitHub CLI")
			}
			return "", fmt.Errorf("gh command not found: install GitHub CLI or set GH_TOKEN")
		}
		return "", fmt.Errorf("gh pr create failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line, nil
		}
	}
	return "", errors.New("gh pr create did not return URL")
}

// AuthStatus は gh auth status をチェックします。トークンがあればフォールバックします。
func (c *Client) AuthStatus(ctx context.Context) error {
	if _, _, err := c.runner.Run(ctx, c.repoDir, "gh", "auth", "status"); err != nil {
		if execx.IsNotFound(err) {
			if c.token != "" {
				return nil
			}
			return fmt.Errorf("gh command not found and no GH_TOKEN/GITHUB_TOKEN available")
		}
		if c.token != "" {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) callGH(ctx context.Context, cmd string, path string) ([]byte, error) {
	args := []string{cmd, path}
	if c.info.Host != "" && !strings.EqualFold(c.info.Host, "github.com") {
		args = append(args, "--hostname", c.info.Host)
	}
	out, stderr, err := c.runner.Run(ctx, c.repoDir, "gh", args...)
	if err != nil {
		if execx.IsNotFound(err) {
			return nil, err
		}
		if len(stderr) > 0 {
			return nil, fmt.Errorf("gh %s failed: %w: %s", cmd, err, strings.TrimSpace(string(stderr)))
		}
		return nil, fmt.Errorf("gh %s failed: %w", cmd, err)
	}
	return out, nil
}

func (c *Client) callREST(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	base := strings.TrimSuffix(c.info.APIBaseURL(), "/")
	endpoint := fmt.Sprintf("%s%s", base, path)
	if query != nil {
		if strings.Contains(endpoint, "?") {
			endpoint += "&" + query.Encode()
		} else {
			endpoint += "?" + query.Encode()
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github api %s %s: %s", method, endpoint, resp.Status)
	}
	return body, nil
}
