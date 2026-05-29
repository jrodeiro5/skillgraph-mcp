package refine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/docs"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/trace"
)

var (
	deepseekEndpoint     = "https://api.deepseek.com/v1/chat/completions"
	geminiEndpoint       = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-lite:generateContent"
	openaiCompatEndpoint = "https://api.openai.com/v1/chat/completions"
)

// StartRefinementLoop starts the background refinement process for servers that
// lack a configured skillGraph, and sets up the SkillOpt trajectory optimization daemon.
func StartRefinementLoop(ctx context.Context, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server) {
	provider, key := getAPIKey()
	if provider == "" {
		slog.Warn("RefinementEngine: No LLM provider configured (set LLM_BASE_URL, OPENAI_API_KEY, DEEPSEEK_API_KEY, or GEMINI_API_KEY). Skipping background graph generation.")
		return
	}

	slog.Info("RefinementEngine: Starting background semantic refinement", "provider", provider)

	// Load the initial config to get the current skillGraph state
	_, initialGraphCfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("RefinementEngine: failed to load initial config to check graph", "error", err)
		return
	}

	// 1. Initial document-based README refinement (Bootstrap phase).
	// Cap concurrent LLM calls and jitter the start of each goroutine so we
	// don't blast the upstream provider with N simultaneous requests, which
	// blows past free-tier rate limits even when total volume is small.
	const bootstrapConcurrency = 3
	sem := make(chan struct{}, bootstrapConcurrency)
	for name := range servers {
		// Check if we already have descriptions or relations defined for this server
		hasDescriptions := false
		if initialGraphCfg != nil && initialGraphCfg.Descriptions != nil {
			if _, ok := initialGraphCfg.Descriptions[name]; ok {
				hasDescriptions = true
			}
		}

		if hasDescriptions {
			slog.Debug("RefinementEngine: Server already has semantic config, skipping bootstrap", "server", name)
			continue
		}

		go func(serverName string) {
			// Stagger startup so concurrent goroutines don't all hit the
			// semaphore at the same instant.
			time.Sleep(time.Duration(rand.IntN(500)) * time.Millisecond)
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if err := refineServer(ctx, provider, key, configPath, mgr, latticeDir, serverName, servers); err != nil {
				slog.Error("RefinementEngine: bootstrap refinement failed", "server", serverName, "error", err)
			}
		}(name)
	}

	// 2. Start SkillOpt background optimization loop (Experience-based refinement)
	go startOptimizationLoop(ctx, provider, key, configPath, mgr, latticeDir, servers)
}

func getAPIKey() (string, string) {
	// Generic OpenAI-compatible provider (LiteLLM proxy, Ollama, etc.) takes priority.
	if os.Getenv("LLM_BASE_URL") != "" {
		return "openai", os.Getenv("LLM_API_KEY")
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return "openai", key
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return "deepseek", key
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return "gemini", key
	}
	return "", ""
}

func refineServer(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir, serverName string, servers map[string]config.Server) error {
	readmePath := filepath.Join(latticeDir, fmt.Sprintf("%s_readme.md", serverName))
	sentinelPath := readmePath + ".noreadme"

	// 1. Wait for either the README to be downloaded or a "no readme available"
	// sentinel from docs.GenerateLattice. Either way, we proceed with refinement —
	// without a README the LLM uses only the tools list.
	start := time.Now()
	var readmeData []byte
	for {
		if data, err := os.ReadFile(readmePath); err == nil {
			readmeData = data
			break
		}
		if _, err := os.Stat(sentinelPath); err == nil {
			slog.Info("RefinementEngine: no README source available, refining from tools only", "server", serverName)
			break
		}
		if time.Since(start) > 15*time.Second {
			slog.Info("RefinementEngine: README fetch timed out, refining from tools only", "server", serverName)
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	// 2. Extract tools and signatures
	tools := mgr.ServerTools(serverName)
	if len(tools) == 0 {
		return fmt.Errorf("no tools found for server %s", serverName)
	}

	var toolsBuilder strings.Builder
	for _, t := range tools {
		fmt.Fprintf(&toolsBuilder, "- Tool Name: %s\n  Description: %s\n  Parameters:\n", t.ResolvedName, t.Description)
		for _, p := range t.Params {
			reqStr := "optional"
			if p.Required {
				reqStr = "required"
			}
			pDesc := ""
			if smap, ok := p.Schema.(map[string]any); ok {
				if d, ok := smap["description"].(string); ok {
					pDesc = d
				}
			}
			if pDesc != "" {
				fmt.Fprintf(&toolsBuilder, "    * %s (%s): %s\n", p.Name, reqStr, pDesc)
			} else {
				fmt.Fprintf(&toolsBuilder, "    * %s (%s)\n", p.Name, reqStr)
			}
		}
		toolsBuilder.WriteString("\n")
	}

	slog.Info("RefinementEngine: Requesting LLM refinement", "server", serverName)

	systemPrompt := `You are an expert software architect building a semantic Model Context Protocol (MCP) capability graph.
Your task is to analyze an MCP server's tools and its README documentation to define its skills, relationships, and override tool/server descriptions.

Output a single JSON object strictly matching this schema:
{
  "descriptions": {
    "<server_or_tool_name>": "A concise, high-quality description explaining what it does and how to use it."
  },
  "relations": [
    {
      "source": "<source_tool_or_server>",
      "target": "<target_tool_or_server>",
      "type": "PREREQUISITE_FOR" | "PRODUCES" | "REQUIRES" | "COMMON_NEXT_STEP",
      "description": "Short explanation of why this relation exists."
    }
  ]
}

Guidelines:
1. The keys in "descriptions" must include the server name itself (explaining the overall capability skill) and can also override specific tools descriptions to make them clearer.
2. Every "source" and "target" in "relations" must exist as a tool name in the provided list or as the server name. Do not hallucinate names.
3. Establish clear prerequisite steps (e.g. tool A produces an output required by tool B) or common next steps (e.g. after search, you common next step is read).
`

	readmeSection := "(no README available — infer from tool names, parameters, and descriptions)"
	if len(readmeData) > 0 {
		readmeSection = string(readmeData)
	}
	userPrompt := fmt.Sprintf("MCP Server Name: %s\n\nREADME Documentation:\n%s\n\nTools List:\n%s", serverName, readmeSection, toolsBuilder.String())

	// 3. Request LLM (with retry on transient errors)
	refinedJSON, err := callLLM(ctx, provider, key, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("calling LLM API: %w", err)
	}

	// 4. Validate and Parse
	var refinedCfg config.SkillGraphConfig
	if err := json.Unmarshal([]byte(refinedJSON), &refinedCfg); err != nil {
		// Attempt a simple recovery (cleaning markdown block ticks if any)
		cleanJSON := cleanMarkdownJSON(refinedJSON)
		if err := json.Unmarshal([]byte(cleanJSON), &refinedCfg); err != nil {
			return fmt.Errorf("invalid JSON returned by LLM: %w. Raw: %s", err, refinedJSON)
		}
	}

	// 5. Semantic Validation (Check for hallucinations)
	validNames := make(map[string]bool)
	validNames[serverName] = true
	for _, t := range tools {
		validNames[t.ResolvedName] = true
	}

	var validRelations []config.StaticRelation
	for _, rel := range refinedCfg.Relations {
		if !validNames[rel.Source] || !validNames[rel.Target] {
			slog.Warn("RefinementEngine: skipping relation containing hallucinated node names", "server", serverName, "source", rel.Source, "target", rel.Target)
			continue
		}
		validRelations = append(validRelations, rel)
	}
	refinedCfg.Relations = validRelations

	// 6. Merge & Save to configPath
	_, currentGraphCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading current config for merge: %w", err)
	}

	if currentGraphCfg.Descriptions == nil {
		currentGraphCfg.Descriptions = make(map[string]string)
	}
	for k, v := range refinedCfg.Descriptions {
		// Only save if the target name is valid (exists as tool or server)
		if validNames[k] {
			currentGraphCfg.Descriptions[k] = v
		}
	}

	// Merge relations avoiding duplicates
	for _, rel := range refinedCfg.Relations {
		dup := false
		for _, existing := range currentGraphCfg.Relations {
			if existing.Source == rel.Source && existing.Target == rel.Target && existing.Type == rel.Type {
				dup = true
				break
			}
		}
		if !dup {
			currentGraphCfg.Relations = append(currentGraphCfg.Relations, rel)
		}
	}

	// Save to config file
	if err := config.SaveSkillGraph(configPath, currentGraphCfg); err != nil {
		return fmt.Errorf("saving refined config: %w", err)
	}

	slog.Info("RefinementEngine: config file successfully updated", "server", serverName)

	// 7. Update memory Graph in Manager
	mgr.RebuildGraph(currentGraphCfg)

	// 8. Regenerate Lattice Markdown files
	if err := docs.GenerateLattice(ctx, latticeDir, servers, mgr.GetGraph()); err != nil {
		slog.Warn("RefinementEngine: failed to regenerate lattice files", "error", err)
	}

	slog.Info("RefinementEngine: successfully integrated refinement for server", "server", serverName)
	return nil
}

// saveSkillGraphSnapshot writes cfg to <historyDir>/skillgraph_<ns>.json and
// trims the directory to the 5 most recent snapshots.
func saveSkillGraphSnapshot(historyDir string, cfg config.SkillGraphConfig) error {
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	name := fmt.Sprintf("skillgraph_%d.json", time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(historyDir, name), data, 0644); err != nil {
		return err
	}

	// Trim to 5 most recent.
	files, err := filepath.Glob(filepath.Join(historyDir, "skillgraph_*.json"))
	if err != nil || len(files) <= 5 {
		return nil
	}
	// Files are named by nanosecond timestamp so lexicographic == chronological.
	for _, f := range files[:len(files)-5] {
		_ = os.Remove(f)
	}
	return nil
}

func cleanMarkdownJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

func callDeepSeek(ctx context.Context, key, system, user string) (string, error) {
	reqBody, err := json.Marshal(map[string]any{
		"model": "deepseek-v4-flash",
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", deepseekEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &HTTPStatusError{Status: resp.StatusCode, Body: "DeepSeek: " + string(body)}
	}

	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Choices) == 0 {
		return "", fmt.Errorf("no choices returned by DeepSeek API")
	}

	return data.Choices[0].Message.Content, nil
}

func callOpenAICompat(ctx context.Context, key, system, user string) (string, error) {
	endpoint := openaiCompatEndpoint
	if base := os.Getenv("LLM_BASE_URL"); base != "" {
		endpoint = strings.TrimRight(base, "/") + "/chat/completions"
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-5.4-nano"
	}

	reqBody, err := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &HTTPStatusError{Status: resp.StatusCode, Body: "OpenAI-compat: " + string(body)}
	}

	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if len(data.Choices) == 0 {
		return "", fmt.Errorf("no choices returned by OpenAI-compat API")
	}
	return data.Choices[0].Message.Content, nil
}

func callGemini(ctx context.Context, key, system, user string) (string, error) {
	promptText := fmt.Sprintf("%s\n\nInput Data:\n%s", system, user)
	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": promptText},
				},
			},
		},
		"generationConfig": map[string]string{
			"responseMimeType": "application/json",
		},
	})
	if err != nil {
		return "", err
	}

	reqURL := fmt.Sprintf("%s?key=%s", geminiEndpoint, key)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &HTTPStatusError{Status: resp.StatusCode, Body: "Gemini: " + string(body)}
	}

	var data struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Candidates) == 0 || len(data.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no candidates returned by Gemini API")
	}

	return data.Candidates[0].Content.Parts[0].Text, nil
}

// splitHoldout partitions files (sorted chronologically) into a hold-out set
// (oldest third) and an optimization set. Returns nil hold-out when fewer than
// 4 files are available — not enough data for meaningful validation.
func splitHoldout(files []string) (holdOut, optimization []string) {
	if len(files) < 4 {
		return nil, files
	}
	n := len(files) / 3
	return files[:n], files[n:]
}

var skipWords = map[string]bool{
	"this": true, "that": true, "with": true, "from": true,
	"have": true, "been": true, "will": true, "when": true,
	"then": true, "your": true, "more": true, "some": true,
	"each": true, "they": true, "them": true, "their": true,
	"used": true, "uses": true, "using": true,
}

// wordSet tokenizes s into a set of lowercase words longer than 3 characters,
// excluding common stop words. Used as a routing-relevance proxy.
func wordSet(s string) map[string]bool {
	words := make(map[string]bool)
	lower := strings.ToLower(s)
	start := -1
	for i, r := range lower {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			if start == -1 {
				start = i
			}
		} else if start != -1 {
			w := lower[start:i]
			if len(w) > 3 && !skipWords[w] {
				words[w] = true
			}
			start = -1
		}
	}
	if start != -1 {
		w := lower[start:]
		if len(w) > 3 && !skipWords[w] {
			words[w] = true
		}
	}
	return words
}

// overlapScore returns the fraction of trace vocabulary (code + tool names)
// that is present in desc. Higher means desc is more relevant to the trace.
func overlapScore(desc string, traj trace.Trajectory) float64 {
	descWords := wordSet(desc)
	if len(descWords) == 0 {
		return 0
	}
	var sb strings.Builder
	sb.WriteString(traj.Code)
	for _, tc := range traj.ToolCalls {
		sb.WriteString(" ")
		sb.WriteString(tc.ToolName)
	}
	taskWords := wordSet(sb.String())
	if len(taskWords) == 0 {
		return 0
	}
	count := 0
	for w := range descWords {
		if taskWords[w] {
			count++
		}
	}
	return float64(count) / float64(len(taskWords))
}

// passesHoldoutGate returns true if replacing current with proposed for toolName
// does not regress routing accuracy on the majority of relevant hold-out traces.
// A trace is "relevant" if it successfully invoked toolName.
// With no hold-out data, every edit is accepted.
func passesHoldoutGate(toolName, proposed, current string, holdOut []trace.Trajectory) bool {
	var relevant []trace.Trajectory
	for _, t := range holdOut {
		for _, tc := range t.ToolCalls {
			if tc.ToolName == toolName && !tc.IsError {
				relevant = append(relevant, t)
				break
			}
		}
	}
	if len(relevant) == 0 {
		return true
	}
	pass := 0
	for _, t := range relevant {
		if overlapScore(proposed, t) >= overlapScore(current, t) {
			pass++
		}
	}
	return pass*2 >= len(relevant)
}

// rollbackThreshold is the minimum relative increase in error rate that triggers
// an automatic rollback to the pre-edit snapshot.
const rollbackThreshold = 1.5

// computeErrRate returns the fraction of traces in files that contain an error.
func computeErrRate(files []string) float64 {
	total, errCount := 0, 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var t trace.Trajectory
		if json.Unmarshal(data, &t) != nil {
			continue
		}
		total++
		if t.Error != "" {
			errCount++
			continue
		}
		for _, tc := range t.ToolCalls {
			if tc.IsError {
				errCount++
				break
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(errCount) / float64(total)
}

// rollbackToLatestSnapshot restores the most recent pre-edit snapshot from
// historyDir into configPath and rebuilds the in-memory graph.
func rollbackToLatestSnapshot(configPath, historyDir string, mgr *mcpserver.Manager) error {
	files, err := filepath.Glob(filepath.Join(historyDir, "skillgraph_*.json"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no snapshots available in %s", historyDir)
	}
	sort.Strings(files) // nanosecond-timestamp prefix: last entry is most recent
	data, err := os.ReadFile(files[len(files)-1])
	if err != nil {
		return fmt.Errorf("reading snapshot: %w", err)
	}
	var cfg config.SkillGraphConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing snapshot: %w", err)
	}
	if err := config.SaveSkillGraph(configPath, &cfg); err != nil {
		return fmt.Errorf("applying snapshot: %w", err)
	}
	mgr.RebuildGraph(&cfg)
	slog.Info("RefinementEngine: rolled back to snapshot", "file", files[len(files)-1])
	return nil
}

func startOptimizationLoop(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	tracesDir := filepath.Join(latticeDir, "traces")
	historyDir := filepath.Join(latticeDir, "history")

	var (
		lastErrRate float64 = -1 // -1 = no baseline established yet
		lastEdited  bool
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if traces directory exists
			if _, err := os.Stat(tracesDir); os.IsNotExist(err) {
				continue
			}

			// Read trace files
			files, err := filepath.Glob(filepath.Join(tracesDir, "*.json"))
			if err != nil || len(files) == 0 {
				continue
			}

			// Limit batch size
			if len(files) > 10 {
				files = files[:10]
			}

			// Sort chronologically (nanosecond-timestamp prefix: lex == time order),
			// then split into hold-out (oldest third) and optimization set.
			sort.Strings(files)
			holdOutFiles, optFiles := splitHoldout(files)

			// Auto-rollback: if the previous cycle made edits and the current
			// batch shows a significantly higher error rate, revert.
			currErrRate := computeErrRate(optFiles)
			if lastEdited && lastErrRate >= 0 && currErrRate > lastErrRate*rollbackThreshold {
				slog.Warn("RefinementEngine: error rate increased after last edit — rolling back",
					"before", lastErrRate, "after", currErrRate)
				if rbErr := rollbackToLatestSnapshot(configPath, historyDir, mgr); rbErr != nil {
					slog.Error("RefinementEngine: rollback failed", "error", rbErr)
				}
				lastEdited = false
				lastErrRate = -1
			}

			edited, err := optimizeTraces(ctx, provider, key, configPath, mgr, latticeDir, servers, optFiles, holdOutFiles)
			if err != nil {
				slog.Error("RefinementEngine: SkillOpt optimization trace failed", "error", err)
			} else if edited {
				lastEdited = true
				lastErrRate = currErrRate
			} else {
				lastEdited = false
			}
		}
	}
}

func optimizeTraces(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server, files []string, holdOutFiles []string) (bool, error) {
	var holdOutTrajectories []trace.Trajectory
	for _, f := range holdOutFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var t trace.Trajectory
		if err := json.Unmarshal(data, &t); err == nil {
			holdOutTrajectories = append(holdOutTrajectories, t)
		}
	}

	var trajectories []trace.Trajectory
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var t trace.Trajectory
		if err := json.Unmarshal(data, &t); err == nil {
			trajectories = append(trajectories, t)
		}
	}

	if len(trajectories) == 0 {
		return false, nil
	}

	// Skip LLM call when the batch has no errors — nothing to optimize.
	hasErrors := false
outer:
	for _, traj := range trajectories {
		if traj.Error != "" {
			hasErrors = true
			break
		}
		for _, tc := range traj.ToolCalls {
			if tc.IsError {
				hasErrors = true
				break outer
			}
		}
	}
	if !hasErrors {
		slog.Debug("RefinementEngine: SkillOpt skipping — no errors in batch", "traces", len(trajectories))
		for _, f := range files {
			_ = os.Remove(f)
		}
		return false, nil
	}

	// 1. Gather all active servers and tools
	tools := mgr.AllTools()
	var toolsBuilder strings.Builder
	validNames := make(map[string]bool)

	for name := range servers {
		validNames[name] = true
	}
	for _, t := range tools {
		validNames[t.ResolvedName] = true
		toolsBuilder.WriteString(fmt.Sprintf("- Tool Name: %s\n  Description: %s\n", t.ResolvedName, t.Description))
	}

	// 2. Format current graph
	g := mgr.GetGraph()
	currentGraphStr := g.FormatCompact()

	// 3. Format traces
	var tracesBuilder strings.Builder
	for i, traj := range trajectories {
		tracesBuilder.WriteString(fmt.Sprintf("\n--- Trace %d ---\n", i+1))
		tracesBuilder.WriteString(fmt.Sprintf("Timestamp: %s\n", traj.Timestamp.Format(time.RFC3339)))
		tracesBuilder.WriteString(fmt.Sprintf("Python Code:\n%s\n", traj.Code))
		if len(traj.ToolCalls) > 0 {
			tracesBuilder.WriteString("Tool Calls Executed:\n")
			for _, tc := range traj.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Args)
				tracesBuilder.WriteString(fmt.Sprintf("  - %s(args: %s) -> Result: %s (Error: %t)\n", tc.ToolName, string(argsJSON), tc.Result, tc.IsError))
			}
		}
		if traj.Error != "" {
			tracesBuilder.WriteString(fmt.Sprintf("Final Execution Error: %s\n", traj.Error))
		} else {
			tracesBuilder.WriteString(fmt.Sprintf("Final Output: %s\n", traj.Output))
		}
	}

	slog.Info("RefinementEngine: Running SkillOpt trace optimizer", "traces", len(trajectories))

	systemPrompt := `You are the SkillOpt optimization engine for a Model Context Protocol (MCP) gateway.
Your task is to analyze recent execution traces (rollouts) of an LLM agent calling tools, reflect on any failures or inefficiencies, and propose optimization edits to the gateway's skill descriptions and relationship graph.

You will receive:
1. The current list of skills and tools.
2. The current capability relationship graph.
3. A list of recent execution traces (code scripts, tool calls made, inputs, outputs, errors).

Look for:
- Runtime errors or TypeError in tool arguments. (Can we improve parameter descriptions or add usage constraints in the tool/server description?)
- Naming confusion (e.g. calling a tool with the wrong name).
- Incorrect order of calls (e.g. calling tool B before tool A is ready). (Can we define a PREREQUISITE_FOR relationship?)

Output a single JSON object strictly matching this schema:
{
  "descriptions": {
    "<server_or_tool_name>": "An optimized description clarifying usage constraints, parameters, or behaviors."
  },
  "relations": [
    {
      "source": "<source_tool_or_server>",
      "target": "<target_tool_or_server>",
      "type": "PREREQUISITE_FOR" | "PRODUCES" | "REQUIRES" | "COMMON_NEXT_STEP",
      "description": "Short explanation of the relation."
    }
  ],
  "delete_relations": [
    {
      "source": "<source_tool_or_server>",
      "target": "<target_tool_or_server>",
      "type": "PREREQUISITE_FOR" | "PRODUCES" | "REQUIRES" | "COMMON_NEXT_STEP"
    }
  ]
}

Guidelines:
1. Only reference actual server names or tool names that exist.
2. Only suggest edits that help resolve or prevent the errors seen in the traces. If there are no issues or room for improvement, return empty lists.
`

	userPrompt := fmt.Sprintf("Current Graph:\n%s\n\nTools:\n%s\n\nRecent Trajectories:\n%s", currentGraphStr, toolsBuilder.String(), tracesBuilder.String())

	refinedJSON, err := callLLM(ctx, provider, key, systemPrompt, userPrompt)
	if err != nil {
		return false, fmt.Errorf("calling LLM API: %w", err)
	}

	type SkillOptEdits struct {
		Descriptions    map[string]string       `json:"descriptions"`
		Relations       []config.StaticRelation `json:"relations"`
		DeleteRelations []config.StaticRelation `json:"delete_relations"`
	}

	var edits SkillOptEdits
	if err := json.Unmarshal([]byte(refinedJSON), &edits); err != nil {
		cleanJSON := cleanMarkdownJSON(refinedJSON)
		if err := json.Unmarshal([]byte(cleanJSON), &edits); err != nil {
			return false, fmt.Errorf("invalid JSON returned by LLM: %w. Raw: %s", err, refinedJSON)
		}
	}

	// Load config for merge
	_, currentGraphCfg, err := config.Load(configPath)
	if err != nil {
		return false, fmt.Errorf("loading current config for merge: %w", err)
	}

	if currentGraphCfg.Descriptions == nil {
		currentGraphCfg.Descriptions = make(map[string]string)
	}

	modified := false
	for k, v := range edits.Descriptions {
		if !validNames[k] {
			continue
		}
		if !passesHoldoutGate(k, v, currentGraphCfg.Descriptions[k], holdOutTrajectories) {
			slog.Warn("RefinementEngine: SkillOpt hold-out gate rejected description edit", "tool", k)
			continue
		}
		currentGraphCfg.Descriptions[k] = v
		modified = true
	}

	for _, rel := range edits.Relations {
		if !validNames[rel.Source] || !validNames[rel.Target] {
			continue
		}
		dup := false
		for _, existing := range currentGraphCfg.Relations {
			if existing.Source == rel.Source && existing.Target == rel.Target && existing.Type == rel.Type {
				dup = true
				break
			}
		}
		if !dup {
			currentGraphCfg.Relations = append(currentGraphCfg.Relations, rel)
			modified = true
		}
	}

	for _, del := range edits.DeleteRelations {
		var keep []config.StaticRelation
		for _, existing := range currentGraphCfg.Relations {
			if existing.Source == del.Source && existing.Target == del.Target && existing.Type == del.Type {
				modified = true
				continue
			}
			keep = append(keep, existing)
		}
		currentGraphCfg.Relations = keep
	}

	if modified {
		// Snapshot current state before overwriting so edits can be rolled back.
		if snapErr := saveSkillGraphSnapshot(filepath.Join(latticeDir, "history"), *currentGraphCfg); snapErr != nil {
			slog.Warn("RefinementEngine: failed to save pre-edit snapshot", "error", snapErr)
		}
		if err := config.SaveSkillGraph(configPath, currentGraphCfg); err != nil {
			return false, fmt.Errorf("saving refined config: %w", err)
		}
		mgr.RebuildGraph(currentGraphCfg)
		if err := docs.GenerateLattice(ctx, latticeDir, servers, mgr.GetGraph()); err != nil {
			slog.Warn("RefinementEngine: failed to regenerate lattice files", "error", err)
		}
		slog.Info("RefinementEngine: SkillOpt applied edits to configuration successfully")
	}

	// Delete all trace files (optimization + hold-out) after successful processing.
	for _, f := range append(files, holdOutFiles...) {
		_ = os.Remove(f)
	}

	return modified, nil
}
