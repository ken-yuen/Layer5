#!/usr/bin/env python3
"""YKC MCP server 最小客戶端 — 驗證 JSON-RPC over stdio（newline-delimited）。"""
import subprocess, json, sys, os

def main():
    server = sys.argv[1] if len(sys.argv) > 1 else "/tmp/ykc-guard"
    project = sys.argv[2] if len(sys.argv) > 2 else "/home/user/demo-rust-cli"

    p = subprocess.Popen([server, "mcp", "-dir", project],
                         stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)

    def req(obj):
        p.stdin.write(json.dumps(obj) + "\n")
        p.stdin.flush()
        return json.loads(p.stdout.readline())

    # 1) initialize
    r = req({"jsonrpc":"2.0","id":1,"method":"initialize",
             "params":{"protocolVersion":"2024-11-05","capabilities":{},
                        "clientInfo":{"name":"py-test","version":"0.1"}}})
    print("initialize →", json.dumps(r.get("result",{}).get("serverInfo")))

    # 2) initialized 通知
    p.stdin.write(json.dumps({"jsonrpc":"2.0","method":"notifications/initialized"}) + "\n")
    p.stdin.flush()

    # 3) tools/list
    r = req({"jsonrpc":"2.0","id":2,"method":"tools/list"})
    tools = r.get("result",{}).get("tools",[])
    names = [t["name"] for t in tools]
    print("tools →", names)
    required = {"ykc.check", "ykc.verify_claims", "ykc.trust_status",
                "ykc.borrow_rules", "ykc.borrow_explain",
                "ykc.kb_search", "ykc.kb_explain"}
    missing = required - set(names)
    if missing:
        raise SystemExit("MCP tools/list missing: " + ", ".join(sorted(missing)))

    # 4) tools/call: ykc.check
    r = req({"jsonrpc":"2.0","id":3,"method":"tools/call",
             "params":{"name":"ykc.check","arguments":{}}})
    print("ykc.check →", r.get("result",{}).get("content",[{}])[0].get("text","")[:80])

    # 5) tools/call: ykc.trust_status
    r = req({"jsonrpc":"2.0","id":4,"method":"tools/call",
             "params":{"name":"ykc.trust_status","arguments":{"agent_id":"agent-honest-01"}}})
    print("ykc.trust_status →", r.get("result",{}).get("content",[{}])[0].get("text",""))

    # 6) tools/call: ykc.kb_search / ykc.kb_explain
    r = req({"jsonrpc":"2.0","id":5,"method":"tools/call",
             "params":{"name":"ykc.kb_search","arguments":{"query":"E0382","k":1}}})
    text = r.get("result",{}).get("content",[{}])[0].get("text","")
    if r.get("result",{}).get("isError") or "E0382" not in text or "KB dataset=" not in text:
        raise SystemExit("ykc.kb_search failed: " + text[:200])

    r = req({"jsonrpc":"2.0","id":6,"method":"tools/call",
             "params":{"name":"ykc.kb_explain","arguments":{"code":"E0382"}}})
    text = r.get("result",{}).get("content",[{}])[0].get("text","")
    if r.get("result",{}).get("isError") or "精確根原子=kb-" not in text:
        raise SystemExit("ykc.kb_explain failed: " + text[:200])
    print("MCP KB tools → E0382 knowledge context verified")

    p.kill()

if __name__ == "__main__":
    main()
