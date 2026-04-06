import json
import subprocess
import sys
import time

process = subprocess.Popen(
    ["go", "run", "./cmd/blueberry/main.go", "-transport=stdio"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=sys.stderr,
    text=True,
    cwd=r"c:\Users\vbode\OneDrive\Desktop\Coding Space\Blueberry"
)

def send(msg):
    process.stdin.write(json.dumps(msg) + "\n")
    process.stdin.flush()

send({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0.0"}}})

# Wait for initialize response
while True:
    line = process.stdout.readline()
    if "jsonrpc" in line and '"id":1' in line:
        break

send({"jsonrpc": "2.0", "method": "notifications/initialized"})

msg = {
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
        "name": "evaluate_argument",
        "arguments": {
            "text": "Artemis II is an ongoing NASA crewed spaceflight mission that launched on April 1, 2026. It is a lunar flyby mission. The crew is Reid Wiseman, Victor Glover, Christina Koch, Jeremy Hansen. As of April 6, 2026, the mission is actively underway and is about two-thirds of the way through its journey.",
            "context_text": "",
            "enrich": True
        }
    }
}
send(msg)

while True:
    line = process.stdout.readline()
    if not line:
        break
    try:
        data = json.loads(line)
        if data.get("id") == 2:
            if "error" in data:
                print("ERROR", data["error"])
            else:
                result = data.get("result", {})
                content = result.get("content", [])
                for c in content:
                    print(c.get("text", ""))
            break
    except Exception as e: # ignore non-json lines
        pass

process.kill()
