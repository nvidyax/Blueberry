import json
import subprocess
import sys
import threading

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

started = False
while True:
    line = process.stdout.readline()
    if 'id":1' in line or 'id": 1' in line:
        started = True
        break

if started:
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})
    send({
        "jsonrpc": "2.0", 
        "id": 2, 
        "method": "tools/call", 
        "params": {
            "name": "evaluate_argument", 
            "arguments": {
                "text": "The moon is made of cheese.", 
                "context_text": "", 
                "enrich": True
            }
        }
    })

    while True:
        line = process.stdout.readline()
        if not line:
            break
        try:
            data = json.loads(line)
            if data.get("id") == 2:
                print("EVALUATE_RESULT:")
                print(json.dumps(data, indent=2))
                break
        except Exception:
            pass

process.kill()
process.wait()
