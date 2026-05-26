package refine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kurtisvg/skillful-mcp/internal/config"
	"github.com/kurtisvg/skillful-mcp/internal/docs"
	"github.com/kurtisvg/skillful-mcp/internal/mcpserver"
)

var (
	deepseekEndpoint = "https://api.deepseek.com/v1/chat/completions"
	geminiEndpoint   = "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent"
)

// StartRefinementLoop starts the background refinement process for servers that
// lack a configured skillGraph, and sets up the SkillOpt trajectory optimization daemon.
func StartRefinementLoop(ctx context.Context, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server) {
	provider, key := getAPIKey()
	if provider == "" || key == "" {
		slog.Warn("RefinementEngine: No DEEPSEEK_API_KEY, GEMINI_API_KEY, or pass key found. Skipping background graph generation.")
		return
	}

	slog.Info("RefinementEngine: Starting background semantic refinement", "provider", provider)

	// Load the initial config to get the current skillGraph state
	_, initialGraphCfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("RefinementEngine: failed to load initial config to check graph", "error", err)
		return
	}

	// 1. Initial document-based README refinement (Bootstrap phase)
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

		// Run refinement in a separate goroutine per server
		go func(serverName string) {
			if err := refineServer(ctx, provider, key, configPath, mgr, latticeDir, serverName, servers); err != nil {
				slog.Error("RefinementEngine: bootstrap refinement failed", "server", serverName, "error", err)
			}
		}(name)
	}

	// 2. Start SkillOpt background optimization loop (Experience-based refinement)
	go startOptimizationLoop(ctx, provider, key, configPath, mgr, latticeDir, servers)
}

func getAPIKey() (string, string) {
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return "deepseek", key
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return "gemini", key
	}

	// Fallback to Pass Manager
	cmd := exec.Command("pass", "show", "deepseek/api_key")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		k := strings.TrimSpace(out.String())
		if k != "" {
			return "deepseek", k
		}
	}

	return "", ""
}

func refineServer(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir, serverName string, servers map[string]config.Server) error {
	readmePath := filepath.Join(latticeDir, fmt.Sprintf("%s_readme.md", serverName))

	// 1. Wait for README to be downloaded by docs.GenerateLattice (up to 15 seconds)
	start := time.Now()
	for {
		if _, err := os.Stat(readmePath); err == nil {
			break
		}
		if time.Since(start) > 15*time.Second {
			return fmt.Errorf("timeout waiting for README file: %s", readmePath)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	readmeData, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading README file: %w", err)
	}

	// 2. Extract tools and signatures
	tools := mgr.ServerTools(serverName)
	if len(tools) == 0 {
		return fmt.Errorf("no tools found for server %s", serverName)
	}

	var toolsBuilder strings.Builder
	for _, t := range tools {
		toolsBuilder.WriteString(fmt.Sprintf("- Tool Name: %s\n  Description: %s\n  Parameters:\n", t.ResolvedName, t.Description))
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
				toolsBuilder.WriteString(fmt.Sprintf("    * %s (%s): %s\n", p.Name, reqStr, pDesc))
			} else {
				toolsBuilder.WriteString(fmt.Sprintf("    * %s (%s)\n", p.Name, reqStr))
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

	userPrompt := fmt.Sprintf("MCP Server Name: %s\n\nREADME Documentation:\n%s\n\nTools List:\n%s", serverName, string(readmeData), toolsBuilder.String())

	// 3. Request LLM
	var refinedJSON string
	if provider == "deepseek" {
		refinedJSON, err = callDeepSeek(ctx, key, systemPrompt, userPrompt)
	} else {
		refinedJSON, err = callGemini(ctx, key, systemPrompt, userPrompt)
	}
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
		"model": "deepseek-chat",
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
		return "", fmt.Errorf("DeepSeek API returned HTTP %d: %s", resp.StatusCode, string(body))
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
		return "", fmt.Errorf("Gemini API returned HTTP %d: %s", resp.StatusCode, string(body))
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

type ToolCallTrace struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
	Result   string         `json:"result"`
	IsError  bool           `json:"is_error"`
}

type Trajectory struct {
	Timestamp time.Time       `json:"timestamp"`
	Code      string          `json:"code"`
	ToolCalls []ToolCallTrace `json:"tool_calls"`
	Output    string          `json:"output"`
	Error     string          `json:"error,omitempty"`
}

func startOptimizationLoop(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	tracesDir := filepath.Join(latticeDir, "traces")

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

			if err := optimizeTraces(ctx, provider, key, configPath, mgr, latticeDir, servers, files); err != nil {
				slog.Error("RefinementEngine: SkillOpt optimization trace failed", "error", err)
			}
		}
	}
}

func optimizeTraces(ctx context.Context, provider, key, configPath string, mgr *mcpserver.Manager, latticeDir string, servers map[string]config.Server, files []string) error {
	var trajectories []Trajectory
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var t Trajectory
		if err := json.Unmarshal(data, &t); err == nil {
			trajectories = append(trajectories, t)
		}
	}

	if len(trajectories) == 0 {
		return nil
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

	var refinedJSON string
	var err error
	if provider == "deepseek" {
		refinedJSON, err = callDeepSeek(ctx, key, systemPrompt, userPrompt)
	} else {
		refinedJSON, err = callGemini(ctx, key, systemPrompt, userPrompt)
	}
	if err != nil {
		return fmt.Errorf("calling LLM API: %w", err)
	}

	type SkillOptEdits struct {
		Descriptions     map[string]string       `json:"descriptions"`
		Relations        []config.StaticRelation `json:"relations"`
		DeleteRelations  []config.StaticRelation `json:"delete_relations"`
	}

	var edits SkillOptEdits
	if err := json.Unmarshal([]byte(refinedJSON), &edits); err != nil {
		cleanJSON := cleanMarkdownJSON(refinedJSON)
		if err := json.Unmarshal([]byte(cleanJSON), &edits); err != nil {
			return fmt.Errorf("invalid JSON returned by LLM: %w. Raw: %s", err, refinedJSON)
		}
	}

	// Load config for merge
	_, currentGraphCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading current config for merge: %w", err)
	}

	if currentGraphCfg.Descriptions == nil {
		currentGraphCfg.Descriptions = make(map[string]string)
	}

	modified := false
	for k, v := range edits.Descriptions {
		if validNames[k] {
			currentGraphCfg.Descriptions[k] = v
			modified = true
		}
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
		if err := config.SaveSkillGraph(configPath, currentGraphCfg); err != nil {
			return fmt.Errorf("saving refined config: %w", err)
		}
		mgr.RebuildGraph(currentGraphCfg)
		if err := docs.GenerateLattice(ctx, latticeDir, servers, mgr.GetGraph()); err != nil {
			slog.Warn("RefinementEngine: failed to regenerate lattice files", "error", err)
		}
		slog.Info("RefinementEngine: SkillOpt applied edits to configuration successfully")
	}

	// Delete trace files after successful processing
	for _, f := range files {
		_ = os.Remove(f)
	}

	return nil
}

