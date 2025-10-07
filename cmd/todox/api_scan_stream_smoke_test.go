package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phyten/todox/internal/engine"
)

func TestAPIScanStreamHandlerEmitsProgressAndResult(t *testing.T) {
	repoDir := prepareStreamRepo(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scan/stream", apiScanStreamHandler(repoDir))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/scan/stream?with_pr_links=0", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to call stream endpoint: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("unexpected content type: %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type progressEvent struct {
		Stage string `json:"stage"`
		Done  int    `json:"done"`
		Total int    `json:"total"`
	}

	var (
		currentEvent string
		dataLines    []string
		progressSeen int
		stages       []string
		gotResult    bool
	)

	flushEvent := func() {
		if currentEvent == "" && len(dataLines) == 0 {
			return
		}
		payload := strings.Join(dataLines, "\n")
		switch currentEvent {
		case "progress":
			var evt progressEvent
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				t.Fatalf("failed to decode progress payload: %v (raw=%s)", err, payload)
			}
			progressSeen++
			stages = append(stages, evt.Stage)
		case "result":
			var res engine.Result
			if err := json.Unmarshal([]byte(payload), &res); err != nil {
				t.Fatalf("failed to decode result payload: %v (raw=%s)", err, payload)
			}
			if len(res.Items) == 0 {
				t.Fatalf("expected result items, got none: %+v", res)
			}
			if res.HasPRs {
				t.Fatalf("with_pr_links=0 should disable PR enrichment")
			}
			gotResult = true
		case "error":
			t.Fatalf("stream returned error event: %s", payload)
		}
		currentEvent = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flushEvent()
			if gotResult {
				break
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(line[6:])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[5:]))
			continue
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		t.Fatalf("stream scan failed: %v", err)
	}

	if progressSeen == 0 {
		t.Fatalf("expected at least one progress event, got 0")
	}
	if !gotResult {
		t.Fatalf("result event was not received")
	}

	for _, stage := range stages {
		if stage == "pr" {
			t.Fatalf("unexpected PR stage when with_pr_links=0: stages=%v", stages)
		}
		switch stage {
		case "scan", "attr", "":
		default:
			t.Fatalf("unknown stage value: %q", stage)
		}
	}
}

func TestAPIScanStreamHandlerStopsOnClientClose(t *testing.T) {
	repoDir := prepareStreamRepo(t)
	handler := apiScanStreamHandler(repoDir)

	pr, pw := io.Pipe()
	recorder := newSSERecorder(pw)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/scan/stream?with_pr_links=0", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		handler(recorder, req)
		close(done)
	}()

	reader := bufio.NewReader(pr)
	sawProgress := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read stream: %v", err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if sawProgress {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, ":") {
			continue
		}
		if strings.HasPrefix(trimmed, "event:") && strings.TrimSpace(trimmed[6:]) == "progress" {
			sawProgress = true
		}
	}
	if !sawProgress {
		t.Fatalf("progress event not observed before cancellation")
	}

	cancel()
	_ = pr.Close()
	_ = pw.CloseWithError(io.ErrClosedPipe)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("handler did not exit after client close")
	}
}

func prepareStreamRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Tester")
	runGit(t, repoDir, "config", "user.email", "tester@example.com")

	source := "package main\n\nfunc main() {\n  // TODO: stream check\n}\n"
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")
	return repoDir
}

type sseResponseRecorder struct {
	header http.Header
	writer io.Writer
	status int
}

func newSSERecorder(w io.Writer) *sseResponseRecorder {
	return &sseResponseRecorder{
		header: make(http.Header),
		writer: w,
		status: http.StatusOK,
	}
}

func (r *sseResponseRecorder) Header() http.Header { return r.header }

func (r *sseResponseRecorder) WriteHeader(status int) { r.status = status }

func (r *sseResponseRecorder) Write(p []byte) (int, error) { return r.writer.Write(p) }

func (r *sseResponseRecorder) Flush() {}
