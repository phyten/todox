package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/phyten/todox/internal/execx"
	"github.com/phyten/todox/internal/gitremote"
	ghclient "github.com/phyten/todox/internal/host/github"
	"github.com/phyten/todox/internal/link"
	"github.com/pkg/browser"
)

func prCmd(args []string) {
	if len(args) == 0 {
		printPrHelp()
		return
	}
	switch args[0] {
	case "find":
		prFind(args[1:])
	case "open":
		prOpen(args[1:])
	case "create":
		prCreate(args[1:])
	case "-h", "--help", "help":
		printPrHelp()
	default:
		fmt.Fprintf(os.Stderr, "todox pr: unknown subcommand %q\n", args[0])
		printPrHelp()
		os.Exit(2)
	}
}

func printPrHelp() {
	fmt.Print("Usage: todox pr <find|open|create> [options]\n\n" +
		"Subcommands:\n" +
		"  find    List pull requests containing a commit\n" +
		"  open    Open the first matching pull request in a browser\n" +
		"  create  Create a pull request via gh CLI\n")
}

func prFind(args []string) {
	fs := flag.NewFlagSet("pr find", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: todox pr find --commit <sha> [--state STATE] [--json] [--repo DIR]")
	}
	commit := fs.String("commit", "", "commit SHA to inspect (required)")
	state := fs.String("state", "all", "filter PRs by state: open|closed|merged|all")
	jsonOut := fs.Bool("json", false, "emit JSON instead of table")
	repoDir := fs.String("repo", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "todox pr find: %v\n", err)
		fs.Usage()
		os.Exit(2)
	}
	if strings.TrimSpace(*commit) == "" {
		fmt.Fprintln(os.Stderr, "todox pr find: --commit is required")
		fs.Usage()
		os.Exit(2)
	}
	ctx := context.Background()
	runner := execx.DefaultRunner()
	info, err := gitremote.Detect(ctx, runner, *repoDir)
	if err != nil {
		log.Fatalf("todox pr find: %v", err)
	}
	client := ghclient.NewClient(info, *repoDir, runner)
	prs, err := client.FindPullRequestsByCommit(ctx, *commit)
	if err != nil {
		log.Fatalf("todox pr find: %v", err)
	}
	filtered, err := filterPRsByState(prs, *state, "--state")
	if err != nil {
		log.Fatalf("todox pr find: %v", err)
	}
	if *jsonOut {
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			log.Fatalf("todox pr find: %v", err)
		}
		fmt.Println(string(data))
		return
	}
	printPRTable(filtered)
}

func prOpen(args []string) {
	fs := flag.NewFlagSet("pr open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: todox pr open --commit <sha> [--state STATE] [--pick N] [--repo DIR]")
	}
	commit := fs.String("commit", "", "commit SHA to inspect (required)")
	state := fs.String("state", "all", "filter PRs by state before opening")
	pick := fs.Int("pick", 1, "1-based index of PR to open")
	repoDir := fs.String("repo", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "todox pr open: %v\n", err)
		fs.Usage()
		os.Exit(2)
	}
	if strings.TrimSpace(*commit) == "" {
		fmt.Fprintln(os.Stderr, "todox pr open: --commit is required")
		fs.Usage()
		os.Exit(2)
	}
	if *pick <= 0 {
		fmt.Fprintln(os.Stderr, "todox pr open: --pick must be >= 1")
		os.Exit(2)
	}
	ctx := context.Background()
	runner := execx.DefaultRunner()
	info, err := gitremote.Detect(ctx, runner, *repoDir)
	if err != nil {
		log.Fatalf("todox pr open: %v", err)
	}
	client := ghclient.NewClient(info, *repoDir, runner)
	prs, err := client.FindPullRequestsByCommit(ctx, *commit)
	if err != nil {
		log.Fatalf("todox pr open: %v", err)
	}
	filtered, err := filterPRsByState(prs, *state, "--state")
	if err != nil {
		log.Fatalf("todox pr open: %v", err)
	}
	if len(filtered) == 0 {
		commitURL := link.Commit(info, *commit)
		if commitURL == "" {
			log.Fatalf("todox pr open: no pull requests found for %s", *commit)
		}
		if err := browser.OpenURL(commitURL); err != nil {
			log.Fatalf("todox pr open: %v", err)
		}
		fmt.Printf("Opened commit %s: %s\n", short(*commit), commitURL)
		return
	}
	if *pick > len(filtered) {
		log.Fatalf("todox pr open: --pick=%d exceeds available PRs (%d)", *pick, len(filtered))
	}
	target := filtered[*pick-1]
	if target.URL == "" {
		log.Fatalf("todox pr open: selected PR has no URL")
	}
	if err := browser.OpenURL(target.URL); err != nil {
		log.Fatalf("todox pr open: %v", err)
	}
	fmt.Printf("Opened %s (#%d, %s)\n", target.URL, target.Number, target.State)
}

func prCreate(args []string) {
	fs := flag.NewFlagSet("pr create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: todox pr create (--commit SHA | --source BRANCH) [--base BRANCH] [--title T] [--body B] [--draft] [--fill] [--yes] [--repo DIR]")
	}
	commit := fs.String("commit", "", "commit SHA to turn into a PR")
	source := fs.String("source", "", "explicit source branch name")
	base := fs.String("base", "", "target branch (default: repo default branch)")
	title := fs.String("title", "", "PR title")
	body := fs.String("body", "", "PR body")
	draft := fs.Bool("draft", false, "create PR as draft")
	fill := fs.Bool("fill", false, "let gh fill title/body from commits")
	yes := fs.Bool("yes", false, "skip interactive prompts")
	repoDir := fs.String("repo", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "todox pr create: %v\n", err)
		fs.Usage()
		os.Exit(2)
	}
	if (*commit == "" && *source == "") || (*commit != "" && *source != "") {
		fmt.Fprintln(os.Stderr, "todox pr create: specify exactly one of --commit or --source")
		fs.Usage()
		os.Exit(2)
	}
	ctx := context.Background()
	runner := execx.DefaultRunner()
	info, err := gitremote.Detect(ctx, runner, *repoDir)
	if err != nil {
		log.Fatalf("todox pr create: %v", err)
	}
	client := ghclient.NewClient(info, *repoDir, runner)
	if err = client.AuthStatus(ctx); err != nil {
		log.Fatalf("todox pr create: %v", err)
	}
	baseBranch := strings.TrimSpace(*base)
	if baseBranch == "" {
		if baseBranch, err = client.DefaultBranch(ctx); err != nil {
			log.Fatalf("todox pr create: failed to resolve default branch: %v", err)
		}
	}
	var sourceBranch string
	if *source != "" {
		sourceBranch = strings.TrimSpace(*source)
	} else {
		var existing []ghclient.PRInfo
		if existing, err = client.FindPullRequestsByCommit(ctx, *commit); err != nil {
			log.Fatalf("todox pr create: %v", err)
		}
		if blocked := blockingPRs(existing); len(blocked) > 0 {
			printBlockingPRs(blocked)
			return
		}
		if sourceBranch, err = inferBranchForCommit(ctx, runner, *repoDir, client, *commit, baseBranch); err != nil {
			log.Fatalf("todox pr create: %v", err)
		}
	}
	if sourceBranch == "" {
		log.Fatalf("todox pr create: could not determine source branch")
	}
	var prsByHead []ghclient.PRInfo
	prsByHead, err = client.FindPullRequestsByHead(ctx, sourceBranch)
	if err != nil && !execx.IsNotFound(err) {
		log.Fatalf("todox pr create: %v", err)
	}
	if blocked := blockingPRs(prsByHead); len(blocked) > 0 {
		printBlockingPRs(blocked)
		return
	}
	extra := make([]string, 0, 6)
	if *title != "" {
		extra = append(extra, "--title", *title)
	}
	if *body != "" {
		extra = append(extra, "--body", *body)
	}
	if *draft {
		extra = append(extra, "--draft")
	}
	if *fill {
		extra = append(extra, "--fill")
	}
	if *yes {
		extra = append(extra, "--yes")
	}
	url, err := client.CreatePullRequest(ctx, sourceBranch, baseBranch, extra)
	if err != nil {
		log.Fatalf("todox pr create: %v", err)
	}
	fmt.Println(url)
}

func filterPRsByState(prs []ghclient.PRInfo, state, flagName string) ([]ghclient.PRInfo, error) {
	norm := strings.ToLower(strings.TrimSpace(state))
	if norm == "" || norm == "all" {
		return prs, nil
	}
	switch norm {
	case "open", "closed", "merged":
		// ok
	default:
		if flagName == "" {
			flagName = "--state"
		}
		return nil, fmt.Errorf("invalid %s: %s", flagName, state)
	}
	out := make([]ghclient.PRInfo, 0, len(prs))
	for _, pr := range prs {
		if strings.EqualFold(pr.State, norm) {
			out = append(out, pr)
		}
	}
	return out, nil
}

func printPRTable(prs []ghclient.PRInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NUMBER\tSTATE\tTITLE\tURL")
	for _, pr := range prs {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", pr.Number, pr.State, pr.Title, pr.URL)
	}
	_ = w.Flush()
}

func blockingPRs(prs []ghclient.PRInfo) []ghclient.PRInfo {
	out := make([]ghclient.PRInfo, 0)
	for _, pr := range prs {
		switch strings.ToLower(pr.State) {
		case "open", "merged":
			out = append(out, pr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

func printBlockingPRs(prs []ghclient.PRInfo) {
	if len(prs) == 0 {
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, "A pull request already exists:")
	for _, pr := range prs {
		_, _ = fmt.Fprintf(os.Stderr, "  #%d [%s] %s\n", pr.Number, pr.State, pr.URL)
	}
}

func inferBranchForCommit(ctx context.Context, runner execx.Runner, repoDir string, client *ghclient.Client, sha, base string) (string, error) {
	names, err := client.BranchesWhereHead(ctx, sha)
	if err == nil && len(names) > 0 {
		filtered := excludeBranch(names, base)
		if len(filtered) == 1 {
			return filtered[0], nil
		}
		if len(filtered) > 1 {
			return "", fmt.Errorf("commit %s matches multiple branches: %s (use --source)", short(sha), strings.Join(filtered, ", "))
		}
	}
	remote, err := parseGitBranches(ctx, runner, repoDir, "git", "branch", "-r", "--contains", sha)
	if err == nil {
		remote = excludeBranch(remote, base)
		if len(remote) == 1 {
			return remote[0], nil
		}
		if len(remote) > 1 {
			return "", fmt.Errorf("commit %s is contained in multiple remote branches: %s (use --source)", short(sha), strings.Join(remote, ", "))
		}
	}
	local, err := parseGitBranches(ctx, runner, repoDir, "git", "branch", "--contains", sha)
	if err == nil {
		local = excludeBranch(local, base)
		if len(local) == 1 {
			return local[0], nil
		}
		if len(local) > 1 {
			return "", fmt.Errorf("commit %s is contained in multiple branches: %s (use --source)", short(sha), strings.Join(local, ", "))
		}
	}
	return "", fmt.Errorf("could not infer source branch for %s; specify --source", short(sha))
}

func parseGitBranches(ctx context.Context, runner execx.Runner, repoDir string, name string, args ...string) ([]string, error) {
	out, stderr, err := runner.Run(ctx, repoDir, name, args...)
	if err != nil {
		if len(stderr) > 0 {
			return nil, fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(stderr)))
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	set := make(map[string]struct{})
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if trimmed == "" || strings.Contains(trimmed, "->") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "origin/")
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	branches := make([]string, 0, len(set))
	for name := range set {
		branches = append(branches, name)
	}
	sort.Strings(branches)
	return branches, nil
}

func excludeBranch(branches []string, base string) []string {
	if base == "" {
		return branches
	}
	out := branches[:0]
	for _, br := range branches {
		if strings.EqualFold(br, base) {
			continue
		}
		out = append(out, br)
	}
	return out
}
