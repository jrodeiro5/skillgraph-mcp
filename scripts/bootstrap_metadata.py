# /// script
# dependencies = [
#   "openai",
# ]
# ///

import subprocess
import json
import sys
import os
from openai import OpenAI

def run_mcp_handshake(command, args, env=None):
    print(f"Connecting to server: {command} {' '.join(args or [])}...", file=sys.stderr)
    full_cmd = [command] + (args or [])
    
    proc = subprocess.Popen(
        full_cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
        env={**os.environ, **(env or {})}
    )
    
    tools = []
    try:
        # 1. Send initialize request
        init_req = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "bootstrap-client", "version": "1.0"}
            }
        }
        proc.stdin.write(json.dumps(init_req) + "\n")
        proc.stdin.flush()
        
        # Read response
        line = proc.stdout.readline()
        if not line:
            return []
        
        # 2. Send initialized notification
        init_notif = {
            "jsonrpc": "2.0",
            "method": "notifications/initialized"
        }
        proc.stdin.write(json.dumps(init_notif) + "\n")
        proc.stdin.flush()
        
        # 3. Send tools/list request
        list_req = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list"
        }
        proc.stdin.write(json.dumps(list_req) + "\n")
        proc.stdin.flush()
        
        # Read response
        line = proc.stdout.readline()
        if line:
            resp = json.loads(line)
            if "result" in resp and "tools" in resp["result"]:
                tools = resp["result"]["tools"]
    except Exception as e:
        print(f"Error communicating with server: {e}", file=sys.stderr)
    finally:
        proc.terminate()
        proc.wait()
    
    print(f"Found {len(tools)} tools.", file=sys.stderr)
    return tools

def refine_description(client, name, kind, schema_or_desc):
    prompt = f"""
    You are an expert AI developer documenting model capabilities for a Model Context Protocol (MCP) server.
    We are implementing a "progressive disclosure" capability graph.
    The agent routing relies on clear, semantic, case-of-use-oriented descriptions of each skill or tool.
    
    Refine the description for the following {kind}:
    Name: {name}
    Current details/schema: {json.dumps(schema_or_desc, indent=2) if isinstance(schema_or_desc, (dict, list)) else schema_or_desc}
    
    Guidelines:
    1. Focus on WHEN and WHY an AI agent should use this {kind} (e.g., "Use this tool when the user wants to...").
    2. Keep it concise (1-2 sentences).
    3. Output ONLY the refined description text. Do not wrap it in quotes, markdown, or any prefix/suffix.
    """
    try:
        response = client.chat.completions.create(
            model="deepseek-chat",
            messages=[{"role": "user", "content": prompt}],
            temperature=0.2
        )
        return response.choices[0].message.content.strip()
    except Exception as e:
        print(f"Error calling DeepSeek API for {name}: {e}", file=sys.stderr)
        return None

def main():
    config_path = "mcp_local.json"
    if not os.path.exists(config_path):
        print(f"Error: {config_path} not found.", file=sys.stderr)
        sys.exit(1)
        
    with open(config_path, "r") as f:
        config_data = json.load(f)
        
    servers = config_data.get("mcpServers", {})
    if not servers:
        print("No servers found in config.", file=sys.stderr)
        sys.exit(1)
        
    # Get API key
    api_key = os.environ.get("DEEPSEEK_API_KEY")
    if not api_key:
        try:
            print("Retrieving DeepSeek API key from pass manager...", file=sys.stderr)
            api_key = subprocess.check_output(["pass", "show", "deepseek/api_key"], text=True).strip()
        except Exception as e:
            print(f"Error: DEEPSEEK_API_KEY is not set and could not retrieve from pass: {e}", file=sys.stderr)
            sys.exit(1)
            
    client = OpenAI(
        api_key=api_key,
        base_url="https://api.deepseek.com/v1"
    )
    
    descriptions = {}
    
    for name, srv in servers.items():
        cmd = srv.get("command")
        args = srv.get("args")
        env = srv.get("env")
        if not cmd:
            continue
            
        current_desc = srv.get("description", "")
        
        tools = run_mcp_handshake(cmd, args, env)
        
        print(f"Refining description for skill: {name}...", file=sys.stderr)
        tool_summaries = [f"- {t['name']}: {t.get('description', '')}" for t in tools]
        skill_context = {
            "current_description": current_desc,
            "available_tools": tool_summaries
        }
        refined_skill = refine_description(client, name, "skill", skill_context)
        if refined_skill:
            descriptions[name] = refined_skill
            print(f"Refined Skill Description: {refined_skill}", file=sys.stderr)
            
        for t in tools:
            resolved_name = t["name"]
            print(f"Refining description for tool: {resolved_name}...", file=sys.stderr)
            tool_context = {
                "name": t["name"],
                "description": t.get("description", ""),
                "input_schema": t.get("inputSchema")
            }
            refined_tool = refine_description(client, resolved_name, "tool", tool_context)
            if refined_tool:
                descriptions[resolved_name] = refined_tool
                descriptions[f"{name}_{resolved_name}"] = refined_tool
                print(f"Refined Tool Description: {refined_tool}", file=sys.stderr)
                
    if "skillGraph" not in config_data:
        config_data["skillGraph"] = {}
    if "descriptions" not in config_data["skillGraph"]:
        config_data["skillGraph"]["descriptions"] = {}
        
    config_data["skillGraph"]["descriptions"].update(descriptions)
    
    with open(config_path, "w") as f:
        json.dump(config_data, f, indent=2)
        
    print(f"Successfully refined {len(descriptions)} descriptions and updated {config_path}!", file=sys.stderr)

if __name__ == "__main__":
    main()
