// Command testreport runs backend API tests and writes a timestamped Markdown report
// (test output summary + per-endpoint case counts + go cover -func).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"log-viewer/backend/internal/apitest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "testreport: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	outDir := filepath.Join(root, "test-results")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	ts := time.Now().Format("20060102-150405")
	coverPath := filepath.Join(outDir, fmt.Sprintf("coverage-%s.out", ts))
	reportPath := filepath.Join(outDir, fmt.Sprintf("api-test-report-%s.md", ts))

	cmd := exec.Command("go", "test", "-count=1", "-json",
		"-cover", "-covermode=atomic", "-coverprofile="+coverPath,
		"./cmd/server/...")
	cmd.Dir = root
	cmd.Env = os.Environ()

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.MultiWriter(os.Stderr, &out)

	testErr := cmd.Run()

	lines := parseTestJSON(out.Bytes())
	summary := summarize(lines)

	coverFunc, coverErr := goToolCoverFunc(root, coverPath)

	md := buildMarkdown(ts, summary, testErr, coverPath, coverFunc, coverErr)

	if err := os.WriteFile(reportPath, []byte(md), 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote report: %s\n", reportPath)
	if testErr != nil {
		return fmt.Errorf("go test failed: %w", testErr)
	}
	return nil
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found from %s", wd)
}

type testEvent struct {
	Time    string `json:"Time"`
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

func parseTestJSON(raw []byte) []testEvent {
	var events []testEvent
	s := bufio.NewScanner(bytes.NewReader(raw))
	for s.Scan() {
		line := s.Bytes()
		var ev testEvent
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		events = append(events, ev)
	}
	return events
}

type leafResult struct {
	Name   string
	Passed bool
	Failed bool
	Skip   bool
}

func summarize(events []testEvent) struct {
	Leaves       []leafResult
	PackagePass  bool
	PackageFail  bool
	FailedPkgs   []string
	TotalElapsed float64
} {
	var r struct {
		Leaves       []leafResult
		PackagePass  bool
		PackageFail  bool
		FailedPkgs   []string
		TotalElapsed float64
	}
	seen := map[string]int{} // full test name -> index in Leaves
	for _, ev := range events {
		if ev.Test == "" {
			if ev.Action == "pass" && ev.Package != "" {
				r.PackagePass = true
				r.TotalElapsed += ev.Elapsed
			}
			if ev.Action == "fail" && ev.Package != "" {
				r.PackageFail = true
				r.FailedPkgs = append(r.FailedPkgs, ev.Package)
			}
			continue
		}
		if !strings.HasPrefix(ev.Test, "TestAPI/") {
			continue
		}
		suffix := strings.TrimPrefix(ev.Test, "TestAPI/")
		if strings.Contains(suffix, "/") {
			continue
		}
		idx, ok := seen[ev.Test]
		if !ok {
			idx = len(r.Leaves)
			seen[ev.Test] = idx
			r.Leaves = append(r.Leaves, leafResult{Name: suffix})
		}
		switch ev.Action {
		case "pass":
			r.Leaves[idx].Passed = true
		case "fail":
			r.Leaves[idx].Failed = true
		case "skip":
			r.Leaves[idx].Skip = true
		}
	}
	sort.Slice(r.Leaves, func(i, j int) bool {
		return r.Leaves[i].Name < r.Leaves[j].Name
	})
	return r
}

func endpointCounts(leaves []leafResult) map[apitest.EndpointID]int {
	m := map[apitest.EndpointID]int{}
	for _, lr := range leaves {
		id, _, ok := strings.Cut(lr.Name, "|")
		if !ok {
			continue
		}
		m[apitest.EndpointID(id)]++
	}
	return m
}

func goToolCoverFunc(root, profile string) (string, error) {
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func buildMarkdown(
	ts string,
	summary struct {
		Leaves       []leafResult
		PackagePass  bool
		PackageFail  bool
		FailedPkgs   []string
		TotalElapsed float64
	},
	testErr error,
	coverPath, coverFunc string,
	coverErr error,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 后端 API 自动化测试报告\n\n")
	fmt.Fprintf(&b, "- 生成时间（时间戳 `%s`）\n", ts)
	fmt.Fprintf(&b, "- 模块: `log-viewer/backend`\n\n")

	fmt.Fprintf(&b, "## 1. 测试执行摘要\n\n")
	passed, failed, skipped := 0, 0, 0
	for _, lr := range summary.Leaves {
		switch {
		case lr.Failed:
			failed++
		case lr.Skip:
			skipped++
		case lr.Passed:
			passed++
		}
	}
	fmt.Fprintf(&b, "| 项目 | 数量 |\n|------|------|\n")
	fmt.Fprintf(&b, "| 子用例总数（按接口维度统计的行） | %d |\n", len(summary.Leaves))
	fmt.Fprintf(&b, "| 通过 | %d |\n", passed)
	fmt.Fprintf(&b, "| 失败 | %d |\n", failed)
	fmt.Fprintf(&b, "| 跳过 | %d |\n", skipped)
	fmt.Fprintf(&b, "| `go test` 总体状态 | %s |\n", overallStatus(summary, testErr))
	if len(summary.FailedPkgs) > 0 {
		fmt.Fprintf(&b, "| 失败包 | %s |\n", strings.Join(summary.FailedPkgs, ", "))
	}
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "### 子用例明细\n\n")
	fmt.Fprintf(&b, "| 结果 | 子用例名 |\n|------|----------|\n")
	for _, lr := range summary.Leaves {
		st := "pass"
		if lr.Failed {
			st = "fail"
		} else if lr.Skip {
			st = "skip"
		}
		fmt.Fprintf(&b, "| %s | `%s` |\n", st, lr.Name)
	}
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "## 2. 接口覆盖（每个接口的测试用例数）\n\n")
	counts := endpointCounts(summary.Leaves)
	var uncovered []string
	for _, e := range apitest.AllEndpointIDs {
		n := counts[e.ID]
		status := "已覆盖"
		if n == 0 {
			status = "**未覆盖**"
			uncovered = append(uncovered, string(e.ID))
		}
		fmt.Fprintf(&b, "- `%s`（%s）: **%d** 个用例 — %s\n", e.Path, e.ID, n, status)
	}
	fmt.Fprintf(&b, "\n")
	if len(uncovered) == 0 {
		fmt.Fprintf(&b, "结论: 所有登记接口均有至少 1 个自动化用例。\n\n")
	} else {
		fmt.Fprintf(&b, "结论: 以下 endpoint id 未出现在任何子用例名中，请补充测试或更新 `internal/apitest/endpoints.go` 登记: **%s**\n\n",
			strings.Join(uncovered, ", "))
	}

	fmt.Fprintf(&b, "## 3. Go 代码覆盖率（`go tool cover -func`）\n\n")
	fmt.Fprintf(&b, "Profile 文件: `%s`\n\n", filepath.ToSlash(coverPath))
	if coverErr != nil {
		fmt.Fprintf(&b, "生成失败: `%v`\n\n", coverErr)
	} else {
		fmt.Fprintf(&b, "```text\n%s\n```\n\n", strings.TrimSpace(coverFunc))
	}

	fmt.Fprintf(&b, "---\n\n由 `go run ./cmd/testreport` 生成。\n")
	return b.String()
}

func overallStatus(summary struct {
	Leaves       []leafResult
	PackagePass  bool
	PackageFail  bool
	FailedPkgs   []string
	TotalElapsed float64
}, testErr error) string {
	if testErr != nil || summary.PackageFail {
		return "失败"
	}
	for _, lr := range summary.Leaves {
		if lr.Failed {
			return "失败"
		}
	}
	return "通过"
}
