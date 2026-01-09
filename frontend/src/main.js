// JS Logic
let currentMode = 'server';
let isRunning = false;

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
        isRunning = running;
        updateServerUI(running);
    });

    window.runtime.EventsOn("client:status", (connected) => {
        isRunning = connected;
        updateClientUI(connected);
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
    
    if (cfg.mode) {
        switchMode(cfg.mode);
    }
}

async function saveConfig() {
    const cfg = {
        mode: currentMode,
        serverPort: parseInt(document.getElementById('serverPort').value),
        serverAddress: document.getElementById('serverAddr').value,
        autoStart: document.getElementById('autoStart').checked
    };
    
    await window.go.main.App.SaveConfig(cfg);
    log("配置已保存");
}

function switchMode(mode) {
    if (isRunning) {
        alert("请先停止当前服务后再切换模式");
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
    
    if (isRunning) {
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
    
    if (isRunning) {
        await window.go.main.App.Disconnect();
    } else {
        const addr = document.getElementById('serverAddr').value;
        if (!addr) return alert("请输入服务端地址");
        
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

function updateClientUI(connected) {
    const btn = document.getElementById('clientToggleBtn');
    const statusBadget = document.getElementById('appStatus');
    const connStatus = document.getElementById('connStatus');
    
    if (connected) {
        btn.innerHTML = '<i class="fa-solid fa-link-slash"></i> 断开连接';
        btn.classList.add('danger');
        btn.classList.remove('primary');
        statusBadget.classList.add('running');
        statusBadget.querySelector('.text').innerText = "已连接";
        connStatus.innerText = "在线";
        connStatus.style.color = "var(--success-color)";
    } else {
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
