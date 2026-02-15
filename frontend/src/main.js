// JS Logic
let currentConfig = {};
let lastCopiedContent = "";
let pollingInterval = null;

window.onload = async () => {
  // Bind JS functions for HTML
  window.switchMode = switchMode;
  window.toggleServer = toggleServer;
  window.toggleClient = toggleClient;
  window.saveConfig = saveConfig;
  window.clearLogs = clearLogs;

  // Load initial config
  try {
    const cfg = await window.go.main.App.GetConfig();
    if (cfg) {
      currentConfig = cfg;
      loadConfigToUI(cfg);
    }
  } catch (e) {
    log("加载配置失败: " + e);
  }

  // Global Error Handler
  window.onerror = function (message, source, lineno, colno, error) {
    log(`❌ 错误: ${message} (${source}:${lineno})`);
    return false;
  };
  window.onunhandledrejection = function (event) {
    log(`❌ 未捕获的 Promise 错误: ${event.reason}`);
  };

  // Listen for SSE status events
  window.runtime.EventsOn("backend:sse:status", (connected) => {
    updateBackendStatus(connected);
  });

  // Start polling status
  startPolling();
};

function startPolling() {
  if (pollingInterval) clearInterval(pollingInterval);
  pollingInterval = setInterval(async () => {
    try {
      const status = await window.go.main.App.GetStatus();
      if (status) {
        updateStatusUI(status);
        
        // Active check for SSE status to cover missed events
        const sseConnected = await window.go.main.App.IsSSEConnected();
        updateBackendStatus(sseConnected);
      }
    } catch (e) {
      console.error("Polling status failed:", e);
      // updateBackendStatus(false); // Handled by SSE event now
      // document.getElementById("appStatus").classList.remove("running");
      // document.getElementById("appStatus").querySelector(".text").innerText = "连接服务失败";
    }
  }, 1000); // Poll every 1 second
}

let isBackendConnected = false;

function updateBackendStatus(connected) {
  if (isBackendConnected === connected) return;
  isBackendConnected = connected;

  const badge = document.getElementById("backendStatus");
  const textSpan = badge.querySelector(".text");
  if (connected) {
    badge.classList.add("connected");
    badge.title = "已连接后台服务 (SSE)";
    if (textSpan) textSpan.innerText = "已连接";
    log("✅ 后台服务已连接 (SSE)");
  } else {
    badge.classList.remove("connected");
    badge.title = "未连接后台服务";
    if (textSpan) textSpan.innerText = "未连接";
    log("❌ 后台服务连接已断开");
  }
}

function updateStatusUI(status) {
  const statusBadget = document.getElementById("appStatus");
  const clientCountSpan = document.getElementById("clientCount");
  const connStatusSpan = document.getElementById("connStatus");

  // Update Last Copied Login
  if (status.last_copied && status.last_copied !== lastCopiedContent) {
    lastCopiedContent = status.last_copied;
    log(`剪贴板更新: ${preview(lastCopiedContent)}`);
  }

  // Update Status Badge
  if (status.running) {
    statusBadget.classList.add("running");
    statusBadget.querySelector(".text").innerText = "运行中";
  } else {
    statusBadget.classList.remove("running");
    statusBadget.querySelector(".text").innerText = "已停止"; // Or "Service Ready" but not syncing
  }

  // Update Mode-specific UI
  if (currentConfig.mode === "server") {
    if (status.running) {
      updateServerBtn(true);
      clientCountSpan.innerText = status.client_count || 0;
    } else {
      updateServerBtn(false);
      clientCountSpan.innerText = "-";
    }
  } else {
    // Client Mode
    // "Running" for client means "Connected" usually, but service says "Running" if autoStart is true?
    // Let's check shared.StatusResponse fields: Running, ClientConnected
    if (status.client_connected) {
      connStatusSpan.innerText = "在线";
      connStatusSpan.style.color = "var(--success-color)";
      updateClientBtn(true);
    } else {
      connStatusSpan.innerText = "离线";
      connStatusSpan.style.color = "var(--text-color)";
      updateClientBtn(false); // If autoStart is true but not connected?
      // If config says autoStart but status says not connected -> connecting/retrying
      if (currentConfig.autoStart) {
        connStatusSpan.innerText = "重连中...";
        connStatusSpan.style.color = "var(--warning-color)";
        updateClientBtn(true); // Button should say "Disconnect"
      }
    }
  }
}

function loadConfigToUI(cfg) {
  document.getElementById("serverPort").value = cfg.serverPort;
  document.getElementById("serverAddr").value = cfg.serverAddress;
  document.getElementById("autoStart").checked = cfg.autoStart;

  // Sync Mode
  const syncMode = cfg.syncMode || "bidirectional";
  const cbSend = document.getElementById("cbSend");
  const cbReceive = document.getElementById("cbReceive");

  if (cbSend && cbReceive) {
    if (syncMode === "bidirectional") {
      cbSend.checked = true;
      cbReceive.checked = true;
    } else if (syncMode === "send_only") {
      cbSend.checked = true;
      cbReceive.checked = false;
    } else if (syncMode === "receive_only") {
      cbSend.checked = false;
      cbReceive.checked = true;
    } else {
      cbSend.checked = false;
      cbReceive.checked = false;
    }
  }

  if (cfg.mode) {
    switchModeUI(cfg.mode);
  }
}

async function saveConfig() {
  const cbSend = document.getElementById("cbSend");
  const cbReceive = document.getElementById("cbReceive");

  let syncMode = "bidirectional";
  if (cbSend && cbReceive) {
    if (cbSend.checked && cbReceive.checked) {
      syncMode = "bidirectional";
    } else if (cbSend.checked && !cbReceive.checked) {
      syncMode = "send_only";
    } else if (!cbSend.checked && cbReceive.checked) {
      syncMode = "receive_only";
    } else {
      syncMode = "disabled";
    }
  }

  const cfg = {
    mode: currentConfig.mode, // Keep current mode
    serverPort: parseInt(document.getElementById("serverPort").value),
    serverAddress: document.getElementById("serverAddr").value,
    autoStart: document.getElementById("autoStart").checked,
    syncMode: syncMode,
  };

  // Optimistic update
  currentConfig = { ...currentConfig, ...cfg };

  try {
    await window.go.main.App.SaveConfig(cfg);
    log("配置已保存");
  } catch (e) {
    log("保存配置失败: " + e);
  }
}

function switchMode(mode) {
  // Prevent switching if service is running
  const statusBadge = document.getElementById("appStatus");
  if (statusBadge.classList.contains("running")) {
    log("⚠️ 阻止切换模式: 服务正在运行");
    showToast("请先停止服务，再切换模式！", "warning");
    return;
  }

  // Update UI state immediately
  document.querySelectorAll(".mode-btn").forEach((btn) => {
    btn.classList.remove("active");
    if (btn.dataset.mode === mode) btn.classList.add("active");
  });

  document.querySelectorAll(".panel").forEach((panel) => {
    panel.classList.remove("active");
  });

  document.getElementById(mode + "Panel").classList.add("active");

  // Save selection temporarily (will save fully on Connect/Start)
  currentConfig.mode = mode;
}

// Pure UI switch — no running check, used for initialization
function switchModeUI(mode) {
  document.querySelectorAll(".mode-btn").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.mode === mode);
  });
  document.getElementById("serverPanel").classList.toggle("active", mode === "server");
  document.getElementById("clientPanel").classList.toggle("active", mode === "client");
}

async function toggleServer() {
  const btn = document.getElementById("serverToggleBtn");
  
  if (isServerRunning) {
    log("正在停止服务...");
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 停止中...';
    serverPending = "stopping";
    try {
      await window.go.main.App.StopSync();
    } catch (e) {
      log("停止服务失败: " + e);
      showToast("停止服务失败: " + e, "error");
      serverPending = null;
      btn.disabled = false;
      updateServerBtn(isServerRunning);
    }
  } else {
    const port = parseInt(document.getElementById("serverPort").value);
    if (!port) {
      showToast("请填写端口号", "warning");
      return;
    }
    log("正在启动服务 (端口: " + port + ") ...");
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 启动中...';
    serverPending = "starting";
    try {
      await window.go.main.App.StartServerMode(port);
    } catch (e) {
      log("启动服务失败: " + e);
      showToast("启动服务失败: " + e, "error");
      serverPending = null;
      btn.disabled = false;
      updateServerBtn(isServerRunning);
    }
    // Button stays in loading state until polling detects running=true
  }
}

async function toggleClient() {
  const btn = document.getElementById("clientToggleBtn");
  const isRunning = btn.classList.contains("danger"); 
  
  if (isRunning) {
    log("正在断开连接...");
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 断开中...';
    clientPending = "stopping";
    try {
      await window.go.main.App.StopSync();
    } catch (e) {
      log("断开连接失败: " + e);
      showToast("断开连接失败: " + e, "error");
      clientPending = null;
      btn.disabled = false;
      updateClientBtn(false);
    }
  } else {
    const addr = document.getElementById("serverAddr").value;
    if (!addr) {
      showToast("请填写服务端地址", "warning");
      return;
    }
    log("正在连接 " + addr + " ...");
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 连接中...';
    clientPending = "starting";
    try {
      await window.go.main.App.StartClientMode(addr);
    } catch (e) {
      log("连接失败: " + e);
      showToast("连接失败: " + e, "error");
      clientPending = null;
      btn.disabled = false;
      updateClientBtn(false);
    }
    // Button stays in loading state until polling detects connected=true
  }
}

// Variables to track state for toggle functions
let isServerRunning = false;
let serverPending = null; // "starting" | "stopping" | null
let clientPending = null; // "starting" | "stopping" | null

function updateServerBtn(running) {
  isServerRunning = running;
  const btn = document.getElementById("serverToggleBtn");

  // If pending, only clear when target state is reached
  if (serverPending === "starting" && running) {
    serverPending = null;
    log("✅ 服务已启动");
  } else if (serverPending === "stopping" && !running) {
    serverPending = null;
    log("✅ 服务已停止");
  } else if (serverPending) {
    return; // Still waiting, keep spinner
  }

  btn.disabled = false;
  if (running) {
    btn.innerHTML = '<i class="fa-solid fa-stop"></i> 停止服务';
    btn.classList.add("danger");
    btn.classList.remove("primary");
  } else {
    btn.innerHTML = '<i class="fa-solid fa-play"></i> 启动服务';
    btn.classList.add("primary");
    btn.classList.remove("danger");
  }
}

function updateClientBtn(running) {
  const btn = document.getElementById("clientToggleBtn");

  // If pending, only clear when target state is reached
  if (clientPending === "starting" && running) {
    clientPending = null;
    log("✅ 已连接服务端");
  } else if (clientPending === "stopping" && !running) {
    clientPending = null;
    log("✅ 已断开连接");
  } else if (clientPending) {
    return; // Still waiting, keep spinner
  }

  btn.disabled = false;
  if (running) {
    btn.innerHTML = '<i class="fa-solid fa-link-slash"></i> 断开连接';
    btn.classList.add("danger");
    btn.classList.remove("primary");
  } else {
    btn.innerHTML = '<i class="fa-solid fa-link"></i> 连接服务端';
    btn.classList.add("primary");
    btn.classList.remove("danger");
  }
}

// Toast Function
function showToast(message, type = "info") {
  try {
    const container = document.getElementById("toast-container");
    if (!container) {
      log("❌ 错误: 找不到 toast-container");
      return;
    }
    const toast = document.createElement("div");
    toast.className = `toast ${type}`;

    let iconClass = "fa-circle-info";
    if (type === "warning") iconClass = "fa-triangle-exclamation";
    if (type === "error") iconClass = "fa-circle-xmark";
    if (type === "success") iconClass = "fa-circle-check";

    toast.innerHTML = `
      <i class="fa-solid ${iconClass}"></i>
      <span>${message}</span>
    `;

    container.appendChild(toast);

    // Auto remove
    setTimeout(() => {
      toast.classList.add("hiding");
      toast.addEventListener("animationend", () => {
        toast.remove();
      });
    }, 3000);
  } catch (e) {
    console.error("Error showing toast:", e);
    log(`❌ 显示通知失败: ${e.message}`);
  }
}

function log(msg) {
  const logs = document.getElementById("logs");
  const entry = document.createElement("div");
  entry.className = "log-entry";

  const time = new Date().toLocaleTimeString();
  entry.innerHTML = `<span class="log-time">[${time}]</span> ${msg}`;

  logs.appendChild(entry);
  logs.scrollTop = logs.scrollHeight;
}

function clearLogs() {
  document.getElementById("logs").innerHTML = "";
}

function preview(str) {
  if (!str) return "";
  return str.length > 20 ? str.substring(0, 20) + "..." : str;
}
