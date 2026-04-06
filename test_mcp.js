const { spawn } = require('child_process');

const proc = spawn('go', ['run', './cmd/blueberry/main.go', '-transport=stdio'], {
  cwd: 'c:\\Users\\vbode\\OneDrive\\Desktop\\Coding Space\\Blueberry',
  env: process.env // inherits environment
});

// To debug stderr
proc.stderr.on('data', data => {
  console.error(`Blueberry Server Log: ${data}`);
});

function send(msg) {
  proc.stdin.write(JSON.stringify(msg) + '\n');
}

let started = false;
let callSent = false;

proc.stdout.on('data', data => {
  const lines = data.toString().split('\n');
  for (const line of lines) {
    if (!line.trim()) continue;
    try {
      const msg = JSON.parse(line);
      if (msg.id === 1) {
        // initialized
        send({ jsonrpc: '2.0', method: 'notifications/initialized' });
        send({
          jsonrpc: '2.0',
          id: 2,
          method: 'tools/call',
          params: {
            name: 'evaluate_argument',
            arguments: {
              text: 'The Artemis II mission launched on April 1, 2026',
              context_text: '',
              enrich: true
            }
          }
        });
        callSent = true;
      } else if (msg.id === 2) {
        console.log('EVALUATE RESULT:');
        console.log(JSON.stringify(msg, null, 2));
        proc.kill();
        process.exit(0);
      }
    } catch (e) {
      // ignore non-json
    }
  }
});

// Kick off
send({
  jsonrpc: '2.0',
  id: 1,
  method: 'initialize',
  params: {
    protocolVersion: '2024-11-05',
    capabilities: {},
    clientInfo: { name: 'test', version: '1.0.0' }
  }
});

setTimeout(() => {
    console.log("Timeout waiting for response. Exiting.");
    proc.kill();
    process.exit(1);
}, 15000);
