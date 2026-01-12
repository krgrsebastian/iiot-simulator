// State
let selectedMachineId = null;
let refreshInterval = null;
let refreshRate = 1000; // Default 1 second
let configUpdateTimeout = null; // For debouncing config updates

// Config Controls DOM Elements
const cycleScaleSlider = document.getElementById('cycle-scale');
const cycleScaleValue = document.getElementById('cycle-scale-value');
const cycleScaleDetail = document.getElementById('cycle-scale-detail');
const scrapRateSlider = document.getElementById('scrap-rate');
const scrapRateValue = document.getElementById('scrap-rate-value');
const errorRateSlider = document.getElementById('error-rate');
const errorRateValue = document.getElementById('error-rate-value');
const configSection = document.getElementById('config-section');

// State code mappings for human-readable display
const STATE_NAMES = {
    0: 'Idle',
    1: 'Setup',
    2: 'Running',
    3: 'PlannedStop',
    4: 'UnplannedStop',
    5: 'Waiting'
};

const GRIPPER_STATE_NAMES = {
    0: 'Open',
    1: 'Closing',
    2: 'Closed',
    3: 'Opening'
};

// Get description for a node (enum values or unit)
function getNodeDescription(nodeName, unit) {
    if (nodeName === 'State') {
        return '0=Idle, 1=Setup, 2=Running, 3=PlannedStop, 4=UnplannedStop, 5=Waiting';
    }
    if (nodeName === 'GripperState') {
        return '0=Open, 1=Closing, 2=Closed, 3=Opening';
    }
    return unit || '-';
}

// DOM Elements
const modeBadge = document.getElementById('mode-badge');
const simulatorName = document.getElementById('simulator-name');
const orderDetails = document.getElementById('order-details');
const machinesGrid = document.getElementById('machines-grid');
const detailSection = document.getElementById('detail-section');
const detailTitle = document.getElementById('detail-title');
const detailContent = document.getElementById('detail-content');
const lastUpdate = document.getElementById('last-update');
const refreshSelect = document.getElementById('refresh-rate');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    fetchStatus();
    fetchMachines();
    fetchConfig();
    startAutoRefresh();
    setupConfigControls();

    // Setup refresh rate picker
    if (refreshSelect) {
        refreshSelect.addEventListener('change', (e) => {
            refreshRate = parseInt(e.target.value);
            restartAutoRefresh();
        });
    }
});

// Auto refresh
function startAutoRefresh() {
    refreshInterval = setInterval(() => {
        fetchStatus();  // Also refresh order info
        fetchMachines();
        if (selectedMachineId) {
            fetchMachineDetail(selectedMachineId);
        }
    }, refreshRate);
}

function restartAutoRefresh() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
    startAutoRefresh();
}

// Fetch simulator status
async function fetchStatus() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();

        modeBadge.textContent = data.mode;
        modeBadge.className = 'mode-badge ' + data.mode;
        simulatorName.textContent = data.simulatorName;

        updateOrderInfo(data.currentOrder);
    } catch (error) {
        console.error('Failed to fetch status:', error);
        modeBadge.textContent = 'Error';
    }
}

// Update order information
function updateOrderInfo(order) {
    if (!order) {
        orderDetails.innerHTML = '<span class="no-order">No active order</span>';
        return;
    }

    const progress = order.quantity > 0
        ? Math.round((order.completed / order.quantity) * 100)
        : 0;

    orderDetails.innerHTML = `
        <div class="order-item">
            <span class="order-label">Order ID</span>
            <span class="order-value">${order.orderId}</span>
        </div>
        <div class="order-item">
            <span class="order-label">Part</span>
            <span class="order-value">${order.partNumber}</span>
        </div>
        <div class="order-item">
            <span class="order-label">Progress</span>
            <span class="order-value">${order.completed} / ${order.quantity} (${progress}%)</span>
        </div>
        <div class="order-item">
            <span class="order-label">Scrap</span>
            <span class="order-value">${order.scrap}</span>
        </div>
        <div class="order-item">
            <span class="order-label">Status</span>
            <span class="order-value">${order.status}</span>
        </div>
    `;
}

// Fetch machines list
async function fetchMachines() {
    try {
        const response = await fetch('/api/machines');
        const data = await response.json();

        renderMachines(data.machines);
        updateLastRefresh();
    } catch (error) {
        console.error('Failed to fetch machines:', error);
    }
}

// Render machine cards
function renderMachines(machines) {
    machinesGrid.innerHTML = machines.map(machine => `
        <div class="machine-card ${selectedMachineId === machine.id ? 'selected' : ''}"
             onclick="selectMachine('${machine.id}')">
            <div class="machine-header">
                <div>
                    <div class="machine-name">${machine.name}</div>
                    <div class="machine-type">${machine.type} (ns=${machine.namespace})</div>
                </div>
                <span class="machine-state ${machine.stateName.toLowerCase()}">${machine.stateName}</span>
            </div>
            <div class="machine-stats">
                <div class="stat">
                    <div class="stat-value good">${machine.goodParts}</div>
                    <div class="stat-label">Good</div>
                </div>
                <div class="stat">
                    <div class="stat-value scrap">${machine.scrapParts}</div>
                    <div class="stat-label">Scrap</div>
                </div>
                <div class="stat">
                    <div class="stat-value progress">${Math.round(machine.cycleProgress)}%</div>
                    <div class="stat-label">Cycle</div>
                </div>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: ${machine.cycleProgress}%"></div>
            </div>
            <div class="machine-footer">Click to view details</div>
        </div>
    `).join('');
}

// Select a machine and show details
function selectMachine(machineId) {
    selectedMachineId = machineId;
    fetchMachineDetail(machineId);

    // Update selected state on cards
    document.querySelectorAll('.machine-card').forEach(card => {
        card.classList.remove('selected');
    });
    event.currentTarget.classList.add('selected');
}

// Fetch machine detail
async function fetchMachineDetail(machineId) {
    try {
        const response = await fetch(`/api/machines/${machineId}`);
        const data = await response.json();

        renderMachineDetail(data);
        detailSection.style.display = 'block';
    } catch (error) {
        console.error('Failed to fetch machine detail:', error);
    }
}

// Render machine detail table
function renderMachineDetail(machine) {
    detailTitle.textContent = `${machine.name} (ns=${machine.namespace})`;

    // Build data rows from both data and nodes
    const rows = machine.nodes.map(node => {
        const value = machine.data[node.name];
        const displayValue = formatValue(value, node.dataType, node.name);
        const nodeId = node.nodeId;
        const description = getNodeDescription(node.name, node.unit);

        return `
            <tr>
                <td>${node.name}</td>
                <td class="value">${displayValue}</td>
                <td class="desc">${description}</td>
                <td class="node-id-cell">
                    <code class="node-id">${nodeId}</code>
                    <button class="copy-btn" onclick="copyToClipboard('${nodeId}', this)" title="Copy Node ID">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                        </svg>
                    </button>
                </td>
                <td>${node.dataType}</td>
            </tr>
        `;
    }).join('');

    detailContent.innerHTML = `
        <table class="data-table">
            <thead>
                <tr>
                    <th>Datapoint</th>
                    <th>Value</th>
                    <th>Desc</th>
                    <th>OPC UA Node ID</th>
                    <th>Type</th>
                </tr>
            </thead>
            <tbody>
                ${rows}
            </tbody>
        </table>
    `;
}

// Copy to clipboard function
async function copyToClipboard(text, button) {
    try {
        await navigator.clipboard.writeText(text);

        // Visual feedback
        const originalHTML = button.innerHTML;
        button.innerHTML = `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
        `;
        button.classList.add('copied');

        setTimeout(() => {
            button.innerHTML = originalHTML;
            button.classList.remove('copied');
        }, 1500);
    } catch (err) {
        console.error('Failed to copy:', err);
        // Fallback for older browsers
        const textArea = document.createElement('textarea');
        textArea.value = text;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
    }
}

// Format value based on data type and node name
function formatValue(value, dataType, nodeName) {
    if (value === undefined || value === null || value === '') {
        return '-';
    }

    // Handle state enums - show human-readable name alongside numeric value
    if (nodeName === 'State' && STATE_NAMES[value] !== undefined) {
        return `${value} (${STATE_NAMES[value]})`;
    }

    if (nodeName === 'GripperState' && GRIPPER_STATE_NAMES[value] !== undefined) {
        return `${value} (${GRIPPER_STATE_NAMES[value]})`;
    }

    if (dataType === 'Double' || dataType === 'Float') {
        return typeof value === 'number' ? value.toFixed(2) : value;
    }

    if (dataType === 'DateTime' && value) {
        try {
            return new Date(value).toLocaleString();
        } catch {
            return value;
        }
    }

    return String(value);
}

// Close detail section
function closeDetail() {
    selectedMachineId = null;
    detailSection.style.display = 'none';

    document.querySelectorAll('.machine-card').forEach(card => {
        card.classList.remove('selected');
    });
}

// Update last refresh timestamp
function updateLastRefresh() {
    const now = new Date();
    lastUpdate.textContent = now.toLocaleTimeString();
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
});

// ==================== Config Controls ====================

// Fetch current config from server
async function fetchConfig() {
    try {
        const response = await fetch('/api/config');
        if (!response.ok) {
            // Config endpoint may not be available in welding robot mode
            if (configSection) {
                configSection.style.display = 'none';
            }
            return;
        }
        const config = await response.json();
        updateConfigUI(config);
        if (configSection) {
            configSection.style.display = 'block';
        }
    } catch (error) {
        console.error('Failed to fetch config:', error);
        if (configSection) {
            configSection.style.display = 'none';
        }
    }
}

// Update config UI elements with current values
function updateConfigUI(config) {
    if (cycleScaleSlider && cycleScaleValue) {
        cycleScaleSlider.value = config.cycleTimeScale;
        cycleScaleValue.textContent = config.cycleTimeScale.toFixed(1) + 'x';
    }
    if (cycleScaleDetail) {
        cycleScaleDetail.textContent = `(${config.effectiveCycleTime} cycles)`;
    }
    if (scrapRateSlider && scrapRateValue) {
        const scrapPercent = Math.round(config.scrapRate * 100);
        scrapRateSlider.value = scrapPercent;
        scrapRateValue.textContent = scrapPercent + '%';
    }
    if (errorRateSlider && errorRateValue) {
        const errorPercent = Math.round(config.errorRate * 100);
        errorRateSlider.value = errorPercent;
        errorRateValue.textContent = errorPercent + '%';
    }
}

// Setup config control event listeners
function setupConfigControls() {
    if (cycleScaleSlider) {
        cycleScaleSlider.addEventListener('input', (e) => {
            const value = parseFloat(e.target.value);
            cycleScaleValue.textContent = value.toFixed(1) + 'x';
            debouncedConfigUpdate({ cycleTimeScale: value });
        });
    }

    if (scrapRateSlider) {
        scrapRateSlider.addEventListener('input', (e) => {
            const percent = parseInt(e.target.value);
            scrapRateValue.textContent = percent + '%';
            debouncedConfigUpdate({ scrapRate: percent / 100 });
        });
    }

    if (errorRateSlider) {
        errorRateSlider.addEventListener('input', (e) => {
            const percent = parseInt(e.target.value);
            errorRateValue.textContent = percent + '%';
            debouncedConfigUpdate({ errorRate: percent / 100 });
        });
    }
}

// Debounce config updates to avoid spamming the server
function debouncedConfigUpdate(updates) {
    if (configUpdateTimeout) {
        clearTimeout(configUpdateTimeout);
    }
    configUpdateTimeout = setTimeout(() => {
        updateConfig(updates);
    }, 300);
}

// Send config update to server
async function updateConfig(updates) {
    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(updates),
        });

        if (!response.ok) {
            const error = await response.text();
            console.error('Config update failed:', error);
            return;
        }

        const config = await response.json();
        // Update the detail text with effective cycle time
        if (cycleScaleDetail) {
            cycleScaleDetail.textContent = `(${config.effectiveCycleTime} cycles)`;
        }
    } catch (error) {
        console.error('Failed to update config:', error);
    }
}
