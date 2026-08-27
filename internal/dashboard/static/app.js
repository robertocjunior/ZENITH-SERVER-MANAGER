// Zenith Protheus Monitor - Vanilla JS Dashboard Controller
// Strictly zero frontend framework, zero canvas, 100% native SVG and Fetch API

(function () {
    'use strict';

    const STATE = {
        cpuHistory: [],
        memHistory: [],
        maxHistoryPoints: 30,
        activeTab: 'overview',
        pollIntervalMs: 3000,
        isPolling: true
    };

    // DOM Elements
    const elements = {
        tabs: document.querySelectorAll('.tab-btn'),
        panes: document.querySelectorAll('.tab-pane'),
        hostBadge: document.getElementById('host-badge'),
        hostName: document.getElementById('target-host-name'),
        lastUpdate: document.getElementById('last-update-time'),
        cpuValue: document.getElementById('cpu-value'),
        cpuBar: document.getElementById('cpu-bar'),
        memValue: document.getElementById('mem-value'),
        memBar: document.getElementById('mem-bar'),
        memDetails: document.getElementById('mem-details'),
        diskValue: document.getElementById('disk-value'),
        diskBar: document.getElementById('disk-bar'),
        diskDetails: document.getElementById('disk-details'),
        portsTableBody: document.getElementById('ports-table-body'),
        processesTableBody: document.getElementById('processes-table-body'),
        logStream: document.getElementById('log-stream'),
        logFilter: document.getElementById('log-filter'),
        tsdbStatus: document.getElementById('tsdb-status-badge'),
        tsdbBufferLen: document.getElementById('tsdb-buffer-len'),
        tsdbDropped: document.getElementById('tsdb-dropped'),
        svgCpuChart: document.getElementById('svg-cpu-chart'),
        svgMemChart: document.getElementById('svg-mem-chart'),
    };

    // Tab Switching
    function initTabs() {
        elements.tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const target = tab.getAttribute('data-tab');
                STATE.activeTab = target;

                elements.tabs.forEach(t => t.classList.remove('active'));
                elements.panes.forEach(p => p.classList.remove('active'));

                tab.classList.add('active');
                const pane = document.getElementById('pane-' + target);
                if (pane) pane.classList.add('active');

                if (target === 'charts') {
                    renderSVGCharts();
                }
            });
        });
    }

    // Format bytes
    function formatBytes(bytes, decimals = 1) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
    }

    // Render SVG Line Chart without using Canvas
    function drawSVGLineChart(svgElement, dataPoints, strokeColor, fillColor) {
        if (!svgElement) return;
        while (svgElement.firstChild) {
            svgElement.removeChild(svgElement.firstChild);
        }

        const width = 600;
        const height = 180;
        const padding = 25;

        svgElement.setAttribute('viewBox', `0 0 ${width} ${height}`);

        if (dataPoints.length < 2) {
            const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            text.setAttribute('x', width / 2);
            text.setAttribute('y', height / 2);
            text.setAttribute('fill', '#64748b');
            text.setAttribute('text-anchor', 'middle');
            text.textContent = 'Coletando amostras de métricas...';
            svgElement.appendChild(text);
            return;
        }

        const chartWidth = width - padding * 2;
        const chartHeight = height - padding * 2;

        // Grid lines
        for (let i = 0; i <= 4; i++) {
            const y = padding + (chartHeight / 4) * i;
            const gridLine = document.createElementNS('http://www.w3.org/2000/svg', 'line');
            gridLine.setAttribute('x1', padding);
            gridLine.setAttribute('y1', y);
            gridLine.setAttribute('x2', width - padding);
            gridLine.setAttribute('y2', y);
            gridLine.setAttribute('stroke', '#2b3345');
            gridLine.setAttribute('stroke-width', '1');
            gridLine.setAttribute('stroke-dasharray', '3,3');
            svgElement.appendChild(gridLine);

            const gridLabel = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            gridLabel.setAttribute('x', padding - 6);
            gridLabel.setAttribute('y', y + 3);
            gridLabel.setAttribute('fill', '#64748b');
            gridLabel.setAttribute('font-size', '10');
            gridLabel.setAttribute('text-anchor', 'end');
            gridLabel.textContent = (100 - i * 25) + '%';
            svgElement.appendChild(gridLabel);
        }

        const stepX = chartWidth / (dataPoints.length - 1);
        let pathD = '';
        let areaD = `M ${padding} ${height - padding}`;

        dataPoints.forEach((val, index) => {
            const clamped = Math.max(0, Math.min(100, val));
            const x = padding + index * stepX;
            const y = padding + chartHeight - (clamped / 100) * chartHeight;

            if (index === 0) {
                pathD += `M ${x} ${y}`;
            } else {
                pathD += ` L ${x} ${y}`;
            }
            areaD += ` L ${x} ${y}`;
        });

        const lastX = padding + (dataPoints.length - 1) * stepX;
        areaD += ` L ${lastX} ${height - padding} Z`;

        // Defs for gradient
        const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
        const grad = document.createElementNS('http://www.w3.org/2000/svg', 'linearGradient');
        const gradId = 'grad-' + Math.random().toString(36).substr(2, 9);
        grad.setAttribute('id', gradId);
        grad.setAttribute('x1', '0%');
        grad.setAttribute('y1', '0%');
        grad.setAttribute('x2', '0%');
        grad.setAttribute('y2', '100%');

        const stop1 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        stop1.setAttribute('offset', '0%');
        stop1.setAttribute('stop-color', strokeColor);
        stop1.setAttribute('stop-opacity', '0.35');

        const stop2 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
        stop2.setAttribute('offset', '100%');
        stop2.setAttribute('stop-color', strokeColor);
        stop2.setAttribute('stop-opacity', '0.0');

        grad.appendChild(stop1);
        grad.appendChild(stop2);
        defs.appendChild(grad);
        svgElement.appendChild(defs);

        // Fill area
        const areaPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        areaPath.setAttribute('d', areaD);
        areaPath.setAttribute('fill', `url(#${gradId})`);
        svgElement.appendChild(areaPath);

        // Stroke line
        const linePath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        linePath.setAttribute('d', pathD);
        linePath.setAttribute('fill', 'none');
        linePath.setAttribute('stroke', strokeColor);
        linePath.setAttribute('stroke-width', '2.5');
        linePath.setAttribute('stroke-linejoin', 'round');
        svgElement.appendChild(linePath);
    }

    function renderSVGCharts() {
        drawSVGLineChart(elements.svgCpuChart, STATE.cpuHistory, '#FF8C00', '#FF8C00');
        drawSVGLineChart(elements.svgMemChart, STATE.memHistory, '#40C4FF', '#40C4FF');
    }

    // Fetch and Update Realtime Telemetry
    async function fetchRealtimeMetrics() {
        try {
            const res = await fetch('/api/v1/metrics/realtime');
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            const data = await res.json();

            // Host & Timestamp
            if (elements.lastUpdate) {
                elements.lastUpdate.textContent = new Date().toLocaleTimeString();
            }

            if (data.host) {
                if (elements.hostName) elements.hostName.textContent = data.host.host;
                if (elements.hostBadge) {
                    if (data.is_mock) {
                        elements.hostBadge.textContent = 'MOCK EMULATOR';
                        elements.hostBadge.className = 'badge badge-mock';
                    } else {
                        elements.hostBadge.textContent = 'AGENTLESS LIVE';
                        elements.hostBadge.className = 'badge badge-live';
                    }
                }

                // CPU
                const cpuVal = data.host.cpu_percent || 0;
                if (elements.cpuValue) elements.cpuValue.textContent = cpuVal.toFixed(1);
                if (elements.cpuBar) {
                    elements.cpuBar.style.width = Math.min(100, Math.max(0, cpuVal)) + '%';
                    elements.cpuBar.className = 'progress-bar-fill' + (cpuVal > 85 ? ' danger' : '');
                }

                // Push to CPU history
                STATE.cpuHistory.push(cpuVal);
                if (STATE.cpuHistory.length > STATE.maxHistoryPoints) {
                    STATE.cpuHistory.shift();
                }

                // RAM
                const memVal = data.host.memory_percent || 0;
                if (elements.memValue) elements.memValue.textContent = memVal.toFixed(1);
                if (elements.memBar) {
                    elements.memBar.style.width = Math.min(100, Math.max(0, memVal)) + '%';
                    elements.memBar.className = 'progress-bar-fill' + (memVal > 85 ? ' danger' : '');
                }
                if (elements.memDetails) {
                    elements.memDetails.textContent = `${formatBytes(data.host.memory_used_bytes)} de ${formatBytes(data.host.memory_total_bytes)}`;
                }

                // Push to RAM history
                STATE.memHistory.push(memVal);
                if (STATE.memHistory.length > STATE.maxHistoryPoints) {
                    STATE.memHistory.shift();
                }

                // Disks
                if (data.host.disks && data.host.disks.length > 0) {
                    const primaryDisk = data.host.disks[0];
                    if (elements.diskValue) elements.diskValue.textContent = primaryDisk.percent.toFixed(1);
                    if (elements.diskBar) {
                        elements.diskBar.style.width = primaryDisk.percent + '%';
                        elements.diskBar.className = 'progress-bar-fill' + (primaryDisk.percent > 90 ? ' danger' : '');
                    }
                    if (elements.diskDetails) {
                        elements.diskDetails.textContent = `${primaryDisk.device} (${formatBytes(primaryDisk.used_bytes)} / ${formatBytes(primaryDisk.total_bytes)})`;
                    }
                }

                // Processes
                renderProcesses(data.host.processes || []);
            }

            // TCP Ports
            renderTCPPorts(data.tcp_ports || []);

            if (STATE.activeTab === 'charts') {
                renderSVGCharts();
            }

        } catch (err) {
            console.error('Error fetching realtime metrics:', err);
        }
    }

    // Render Monitored Processes Table
    function renderProcesses(processes) {
        if (!elements.processesTableBody) return;
        elements.processesTableBody.innerHTML = '';

        if (processes.length === 0) {
            elements.processesTableBody.innerHTML = '<tr><td colspan="7" style="text-align:center; color:#64748b;">Nenhum processo TOTVS Protheus ativo detectado.</td></tr>';
            return;
        }

        processes.forEach(p => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td><strong style="color:var(--accent-orange);">${p.name}</strong></td>
                <td><code>${p.pid}</code></td>
                <td>${p.cpu_percent ? p.cpu_percent.toFixed(1) + '%' : '0.0%'}</td>
                <td>${formatBytes(p.working_set_bytes)}</td>
                <td>${p.thread_count || '-'}</td>
                <td>${p.handle_count || '-'}</td>
                <td><span class="badge ${p.status === 'RUNNING' ? 'badge-live' : 'badge-mock'}">${p.status}</span></td>
            `;
            elements.processesTableBody.appendChild(tr);
        });
    }

    // Render TCP Ports Table
    function renderTCPPorts(ports) {
        if (!elements.portsTableBody) return;
        elements.portsTableBody.innerHTML = '';

        if (ports.length === 0) {
            elements.portsTableBody.innerHTML = '<tr><td colspan="5" style="text-align:center; color:#64748b;">Nenhuma porta TCP configurada.</td></tr>';
            return;
        }

        ports.forEach(p => {
            const tr = document.createElement('tr');
            const statusHtml = p.up
                ? `<span class="port-status-up"><span class="pulse-dot"></span>ONLINE</span>`
                : `<span class="port-status-down">OFFLINE</span>`;

            tr.innerHTML = `
                <td><strong>${p.name}</strong></td>
                <td><code>${p.port}</code></td>
                <td>${statusHtml}</td>
                <td>${p.latency_ms ? p.latency_ms.toFixed(2) + ' ms' : '-'}</td>
                <td style="color:#64748b; font-size:0.75rem;">${p.error ? p.error : 'OK'}</td>
            `;
            elements.portsTableBody.appendChild(tr);
        });
    }

    // Fetch and Append Logs
    async function fetchLogs() {
        try {
            const res = await fetch('/api/v1/logs');
            if (!res.ok) return;
            const logs = await res.json();
            renderLogs(logs);
        } catch (err) {
            console.error('Error fetching logs:', err);
        }
    }

    function renderLogs(logs) {
        if (!elements.logStream) return;
        elements.logStream.innerHTML = '';

        if (!logs || logs.length === 0) {
            elements.logStream.innerHTML = '<div style="color:#64748b; text-align:center; padding:2rem;">Nenhum evento recente de log registrado.</div>';
            return;
        }

        const filter = elements.logFilter ? elements.logFilter.value : 'ALL';

        logs.forEach(log => {
            if (filter !== 'ALL' && log.level !== filter) return;

            const timeStr = new Date(log.timestamp).toLocaleTimeString();
            const div = document.createElement('div');
            div.className = `log-entry ${log.level}`;
            div.innerHTML = `
                <span class="log-time">${timeStr}</span>
                <span class="log-tag ${log.level}">${log.level}</span>
                <span class="log-cat">[${log.category}]</span>
                <span class="log-msg">${escapeHtml(log.message)}</span>
            `;
            elements.logStream.appendChild(div);
        });
    }

    // Fetch Status & TSDB telemetry
    async function fetchStatus() {
        try {
            const res = await fetch('/api/v1/status');
            if (!res.ok) return;
            const data = await res.json();

            if (elements.tsdbStatus) {
                if (data.tsdb_healthy) {
                    elements.tsdbStatus.textContent = 'CONNECTED';
                    elements.tsdbStatus.className = 'badge badge-live';
                } else {
                    elements.tsdbStatus.textContent = 'OFFLINE / RETRYING';
                    elements.tsdbStatus.className = 'badge badge-mock';
                }
            }
            if (elements.tsdbBufferLen) elements.tsdbBufferLen.textContent = data.tsdb_buffer_len;
            if (elements.tsdbDropped) elements.tsdbDropped.textContent = data.tsdb_dropped;
        } catch (err) {
            console.error('Error fetching status:', err);
        }
    }

    function escapeHtml(str) {
        if (!str) return '';
        return str
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    // Polling Controller
    function startPolling() {
        fetchRealtimeMetrics();
        fetchLogs();
        fetchStatus();

        setInterval(() => {
            if (STATE.isPolling) {
                fetchRealtimeMetrics();
                fetchLogs();
                fetchStatus();
            }
        }, STATE.pollIntervalMs);
    }

    // Initial setup
    document.addEventListener('DOMContentLoaded', () => {
        initTabs();
        if (elements.logFilter) {
            elements.logFilter.addEventListener('change', () => {
                fetchLogs();
            });
        }
        startPolling();
    });

})();
