package link

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/phyten/todox/internal/gitremote"
)

// Blob はコミット SHA とファイルパス、行番号から GitHub 互換の blob URL を生成します。
func Blob(info gitremote.Info, sha, file string, line int) string {
	if sha == "" || file == "" || line <= 0 {
		return ""
	}
	path := gitremote.BlobPath(file)
	host := strings.TrimSuffix(info.Host, "/")
	owner := url.PathEscape(info.Owner)
	repo := url.PathEscape(info.Repo)
	scheme := info.NormalizedScheme()
	if isMarkdown(file) {
		return fmt.Sprintf("%s://%s/%s/%s/blob/%s/%s?plain=1#L%d", scheme, host, owner, repo, sha, path, line)
	}
	return fmt.Sprintf("%s://%s/%s/%s/blob/%s/%s#L%d", scheme, host, owner, repo, sha, path, line)
}

// Commit はコミット詳細ページの URL を返します。
func Commit(info gitremote.Info, sha string) string {
	if sha == "" {
		return ""
	}
	host := strings.TrimSuffix(info.Host, "/")
	owner := url.PathEscape(info.Owner)
	repo := url.PathEscape(info.Repo)
	scheme := info.NormalizedScheme()
	return fmt.Sprintf("%s://%s/%s/%s/commit/%s", scheme, host, owner, repo, sha)
}

func isMarkdown(file string) bool {
	lower := strings.ToLower(file)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}
