const { spawn } = require('child_process');
const fs = require('fs');

const targetText = fs.readFileSync("claims.txt", "utf8");

// read IDE mcp_config.json to grab env overrides
let mcpEnv = {};
try {
    const configPath = "C:\\Users\\vbode\\.gemini\\antigravity\\mcp_config.json";
    const mcpConfig = JSON.parse(fs.readFileSync(configPath, 'utf8'));
    if (mcpConfig.mcpServers && mcpConfig.mcpServers.blueberry && mcpConfig.mcpServers.blueberry.env) {
        mcpEnv = mcpConfig.mcpServers.blueberry.env;
    }
} catch(e) {
    console.error("Warning: could not read mcp_config.json envs", e);
}

const finalEnv = Object.assign({}, process.env, mcpEnv);

const proc = spawn('go', ['run', './cmd/blueberry/main.go', '-transport', 'stdio'], {
    cwd: 'C:\\Users\\vbode\\OneDrive\\Desktop\\Coding Space\\Blueberry',
    env: finalEnv
});

let outputBuffer = "";

proc.stdout.on('data', data => {
    outputBuffer += data.toString();
    
    let parts = outputBuffer.split('\n');
    outputBuffer = parts.pop(); 
    
    for (let line of parts) {
        if (!line.trim()) continue;
        try {
            const msg = JSON.parse(line);
            if (msg.id === 1) {
                proc.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
                
                const getReq = {
                    jsonrpc: '2.0',
                    id: 2,
                    method: 'tools/call',
                    params: {
                        name: 'evaluate_argument',
                        arguments: {
                            text: targetText,
                            context_text: '',
                            enrich: true
                        }
                    }
                };
                proc.stdin.write(JSON.stringify(getReq) + '\n');
            } else if (msg.id === 2) {
                console.log(JSON.stringify(msg, null, 2));
                proc.kill();
                process.exit(0);
            } else if (msg.error) {
                console.error("MCP_ERROR:", JSON.stringify(msg.error));
                proc.kill();
                process.exit(1);
            }
        } catch (e) {}
    }
});

const initReq = {
    jsonrpc: '2.0', id: 1,
    method: 'initialize',
    params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'test', version: '1.0.0' } }
};

proc.stdin.write(JSON.stringify(initReq) + '\n');

setTimeout(() => {
    console.error('MCP_ERROR: Timed out waiting for response.');
    proc.kill();
    process.exit(1);
}, 45000); // 45 seconds to allow LLM to respond
