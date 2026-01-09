// JS Logic
let currentMode = 'server';
// Server running state (true/false)
let isServerRunning = false;
// Client user intent (true = user pressed Connect, false = user pressed Disconnect)
let isClientIntentRunning = false;
// Client actual connection status (true/false)
let isClientConnected = false;

window.onload = async () => {
    // 绑定 JS 函数到全局以便 HTML 调用
    window.switchMode = switchMode;
    window.toggleServer = toggleServer;
    window.toggleClient = toggleClient;
    window.saveConfig = saveConfig;
    window.clearLogs = clearLogs;

    // 初始化事件监听
    setupEvents();
    
    // 加载配置
    try {
        const cfg = await window.go.main.App.GetConfig();
        loadConfigToUI(cfg);
    } catch (e) {
        log("加载配置失败: " + e);
    }
};

function setupEvents() {
    // 监听运行时事件
    window.runtime.EventsOn("log", log);
    window.runtime.EventsOn("status", updateStatus);
    
    window.runtime.EventsOn("server:running", (running) => {
        isServerRunning = running;
        updateServerUI(running);
    });

    window.runtime.EventsOn("client:status", (connected) => {
        isClientConnected = connected;
        // 如果连接成功，确保意图状态也为运行中（防止状态不一致）
        if (connected) {
            isClientIntentRunning = true;
        }
        updateClientUI(isClientIntentRunning, connected);
    });

    window.runtime.EventsOn("server:client_count", (count) => {
        document.getElementById('clientCount').innerText = count;
    });

    window.runtime.EventsOn("clipboard:local", (content) => {
        log(`本地复制: ${preview(content)}`);
    });

    window.runtime.EventsOn("clipboard:remote", (content) => {
        log(`收到同步: ${preview(content)}`);
    });
}

function loadConfigToUI(cfg) {
    document.getElementById('serverPort').value = cfg.serverPort;
    document.getElementById('serverAddr').value = cfg.serverAddress;
    document.getElementById('autoStart').checked = cfg.autoStart;
    
    // 加载同步模式
    const syncMode = cfg.syncMode || 'bidirectional';
    const cbSend = document.getElementById('cbSend');
    const cbReceive = document.getElementById('cbReceive');

    if (cbSend && cbReceive) {
        if (syncMode === 'bidirectional') {
            cbSend.checked = true;
            cbReceive.checked = true;
        } else if (syncMode === 'send_only') {
            cbSend.checked = true;
            cbReceive.checked = false;
        } else if (syncMode === 'receive_only') {
            cbSend.checked = false;
            cbReceive.checked = true;
        } else {
            // disabled or unknown
            cbSend.checked = false;
            cbReceive.checked = false;
        }
    }

    if (cfg.mode) {
        switchMode(cfg.mode);
    }
}

async function saveConfig() {
    const cbSend = document.getElementById('cbSend');
    const cbReceive = document.getElementById('cbReceive');
    
    let syncMode = 'bidirectional';
    if (cbSend && cbReceive) {
        if (cbSend.checked && cbReceive.checked) {
            syncMode = 'bidirectional';
        } else if (cbSend.checked && !cbReceive.checked) {
            syncMode = 'send_only';
        } else if (!cbSend.checked && cbReceive.checked) {
            syncMode = 'receive_only';
        } else {
            // 都没选，暂时当做 bidirectional 或者 disabled?
            // 由于后端没有 disabled 逻辑，且用户可能误操作，我们可以定义一个 disabled 状态
            // 或者默认 bidirectional.
            // 这里为了安全，如果全取消，就什么都不做 (disabled)
            syncMode = 'disabled'; 
        }
    }

    const cfg = {
        mode: currentMode,
        serverPort: parseInt(document.getElementById('serverPort').value),
        serverAddress: document.getElementById('serverAddr').value,
        autoStart: document.getElementById('autoStart').checked,
        syncMode: syncMode
    };
    
    await window.go.main.App.SaveConfig(cfg);
    log("配置已保存");
}

function switchMode(mode) {
    if (currentMode === 'server' && isServerRunning) {
        alert("请先停止服务端后再切换模式");
        return;
    }
    // 客户端模式下，允许随时切换，但最好提示一下
    if (currentMode === 'client' && isClientIntentRunning) {
        alert("请先断开连接后再切换模式");
        return;
    }

    currentMode = mode;
    
    // UI 切换
    document.querySelectorAll('.mode-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.mode === mode);
    });
    
    document.getElementById('serverPanel').classList.toggle('active', mode === 'server');
    document.getElementById('clientPanel').classList.toggle('active', mode === 'client');
    
    saveConfig();
}

async function toggleServer() {
    const btn = document.getElementById('serverToggleBtn');
    
    if (isServerRunning) {
        await window.go.main.App.StopServer();
    } else {
        const port = parseInt(document.getElementById('serverPort').value);
        if (!port) return alert("请输入有效端口");
        
        await saveConfig(); // 启动前保存配置
        await window.go.main.App.StartServer(port);
    }
}

async function toggleClient() {
    const btn = document.getElementById('clientToggleBtn');
    
    if (isClientIntentRunning) {
        // 用户点击断开连接
        isClientIntentRunning = false;
        // 立即更新 UI 状态，虽然 disconnect 也是异步的
        updateClientUI(false, isClientConnected);
        await window.go.main.App.Disconnect();
    } else {
        // 用户点击连接
        const addr = document.getElementById('serverAddr').value;
        if (!addr) return alert("请输入服务端地址");
        
        isClientIntentRunning = true;
        updateClientUI(true, false); // 意图为运行，连接状态暂时未知/连接中

        await saveConfig(); // 启动前保存配置
        await window.go.main.App.ConnectToServer(addr);
    }
}

function updateServerUI(running) {
    const btn = document.getElementById('serverToggleBtn');
    const statusBadget = document.getElementById('appStatus');
    
    if (running) {
        btn.innerHTML = '<i class="fa-solid fa-stop"></i> 停止服务';
        btn.classList.add('danger');
        btn.classList.remove('primary');
        statusBadget.classList.add('running');
        statusBadget.querySelector('.text').innerText = "运行中";
    } else {
        btn.innerHTML = '<i class="fa-solid fa-play"></i> 启动服务';
        btn.classList.add('primary');
        btn.classList.remove('danger');
        statusBadget.classList.remove('running');
        statusBadget.querySelector('.text').innerText = "已停止";
        document.getElementById('clientCount').innerText = "0";
    }
}

function updateClientUI(intentRunning, connected) {
    const btn = document.getElementById('clientToggleBtn');
    const statusBadget = document.getElementById('appStatus');
    const connStatus = document.getElementById('connStatus');
    
    if (intentRunning) {
        // 用户想连接 / 正在运行
        // 按钮显示“断开连接”
        btn.innerHTML = '<i class="fa-solid fa-link-slash"></i> 断开连接';
        btn.classList.add('danger');
        btn.classList.remove('primary');

        if (connected) {
            statusBadget.classList.add('running');
            statusBadget.querySelector('.text').innerText = "已连接";
            connStatus.innerText = "在线";
            connStatus.style.color = "var(--success-color)";
        } else {
            // 连接断开，但意图是运行，显示重连中
            statusBadget.classList.remove('running'); // 或者用另一个颜色
            statusBadget.querySelector('.text').innerText = "重连中";
            connStatus.innerText = "断开 (重连中...)";
            connStatus.style.color = "var(--warning-color)"; // 需要在 css 定义或直接使用 orange
        }
    } else {
        // 用户已停止
        btn.innerHTML = '<i class="fa-solid fa-link"></i> 连接服务端';
        btn.classList.add('primary');
        btn.classList.remove('danger');
        statusBadget.classList.remove('running');
        statusBadget.querySelector('.text').innerText = "未连接";
        connStatus.innerText = "离线";
        connStatus.style.color = "var(--text-color)";
    }
}

function updateStatus(msg) {
    log(msg);
}

function log(msg) {
    const logs = document.getElementById('logs');
    const entry = document.createElement('div');
    entry.className = 'log-entry';
    
    const time = new Date().toLocaleTimeString();
    entry.innerHTML = `<span class="log-time">[${time}]</span> ${msg}`;
    
    logs.appendChild(entry);
    logs.scrollTop = logs.scrollHeight;
}

function clearLogs() {
    document.getElementById('logs').innerHTML = '';
}

function preview(str) {
    if (!str) return "";
    return str.length > 20 ? str.substring(0, 20) + "..." : str;
}
