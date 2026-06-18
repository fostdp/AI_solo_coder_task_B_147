const VirtualMaintenance = {
    materials: [],
    lubricants: [],
    bearingId: 1,
    currentBearing: null,
    currentStatus: null,
    maintenanceHistory: [],
    sessionId: "visitor-" + Math.random().toString(36).slice(2, 10),
    operatorName: null,
    totalSavings: 0,
    actionsTaken: 0,

    init(bearingId = 1) {
        this.bearingId = bearingId;
        this.loadReferenceData();
        this.loadBearingInfo();
        this.bindEvents();
        this.loadMaintenanceHistory();
    },

    async loadReferenceData() {
        try {
            const [matRes, lubRes] = await Promise.all([
                API.getMaterials(),
                API.getLubricants(),
            ]);
            this.materials = matRes.data || [];
            this.lubricants = lubRes.data || [];
            this.renderReplaceMaterials();
            this.renderLubricantOptions();
        } catch (e) {
            console.error("加载参考数据失败:", e);
        }
    },

    async loadBearingInfo() {
        try {
            const [bearing, statuses, plan] = await Promise.all([
                API.getBearingById(this.bearingId),
                API.getBearingStatuses(),
                API.getMaintenancePlan(this.bearingId).catch(() => null),
            ]);
            this.currentBearing = bearing;
            this.currentStatus = statuses.find(s => s.bearing_id === this.bearingId) || statuses.find(s => s.BearingID === this.bearingId) || null;
            this.renderBearingDashboard();
            if (plan) this.renderMaintenancePlan(plan);
        } catch (e) {
            console.error("加载轴承信息失败:", e);
        }
    },

    async loadMaintenanceHistory() {
        try {
            const res = await API.getMaintenanceHistory(0, 20);
            this.maintenanceHistory = res.data || [];
            this.renderHistory();
        } catch (e) {
            console.error("加载维护历史失败:", e);
        }
    },

    bindEvents() {
        const btnReplacePreview = document.getElementById("btn-preview-replace");
        if (btnReplacePreview) btnReplacePreview.onclick = () => this.previewReplacement();

        const btnReplaceExecute = document.getElementById("btn-execute-replace");
        if (btnReplaceExecute) btnReplaceExecute.onclick = () => this.executeReplacement();

        const btnLubePreview = document.getElementById("btn-preview-lube");
        if (btnLubePreview) btnLubePreview.onclick = () => this.previewLubrication();

        const btnLubeExecute = document.getElementById("btn-execute-lube");
        if (btnLubeExecute) btnLubeExecute.onclick = () => this.executeLubrication();

        const nameInput = document.getElementById("operator-name-input");
        if (nameInput) {
            nameInput.onchange = (e) => {
                this.operatorName = e.target.value.trim() || null;
            };
        }
    },

    renderBearingDashboard() {
        const b = this.currentBearing;
        const s = this.currentStatus;
        if (!b || !s) return;

        const infoDiv = document.getElementById("bearing-info-panel");
        if (!infoDiv) return;

        const wearUm = s.total_wear_microm || s.TotalWearMicrom || 0;
        const wearLimit = b.wear_limit_microm || b.WearLimitMicrom || 500;
        const wearPct = (wearUm / wearLimit * 100).toFixed(1);
        const rul = s.predicted_rul_hours || s.PredictedRULHours || 0;
        const rulYears = (rul / 8760).toFixed(1);
        const reliab = s.reliability || s.Reliability || 0;

        const wearStatusClass = wearPct > 90 ? "critical" : wearPct > 70 ? "warning" : "healthy";
        const healthColor = s.health_status === "critical" || s.HealthStatus === "critical" ? "#ef5350"
            : s.health_status === "warning" || s.HealthStatus === "warning" ? "#ffa726" : "#66bb6a";

        infoDiv.innerHTML = `
            <div class="bearing-info-header">
                <div class="info-title">
                    <h3>${b.bearing_code || b.BearingCode}</h3>
                    <div class="info-sub">${b.position || b.Position} · ${b.material || b.Material}</div>
                </div>
                <div class="health-badge" style="background:${healthColor}">
                    ${s.health_status || s.HealthStatus || "unknown"}
                </div>
            </div>
            <div class="info-stats">
                <div class="info-stat">
                    <div class="stat-label">当前磨损</div>
                    <div class="stat-value wear-${wearStatusClass}">${wearUm.toFixed(2)} μm</div>
                    <div class="stat-sub">限值 ${wearLimit}μm (${wearPct}%)</div>
                    <div class="wear-progress">
                        <div class="wear-fill wear-${wearStatusClass}" style="width:${Math.min(100, wearPct)}%"></div>
                        <div class="warn-line" style="left:70%"></div>
                        <div class="crit-line" style="left:90%"></div>
                    </div>
                </div>
                <div class="info-stat">
                    <div class="stat-label">剩余寿命</div>
                    <div class="stat-value">${rul.toFixed(0)} h</div>
                    <div class="stat-sub">约 ${rulYears} 年</div>
                    <div class="reliability-ring" style="--pct:${reliab}">
                        <span>${(reliab * 100).toFixed(0)}%</span>
                    </div>
                </div>
                <div class="info-stat">
                    <div class="stat-label">实时工况</div>
                    <div class="stat-mini-row">
                        <span>温度</span>
                        <b>${(s.temperature || s.Temperature || 0).toFixed(1)}°C</b>
                    </div>
                    <div class="stat-mini-row">
                        <span>载荷</span>
                        <b>${(s.radial_load || s.RadialLoad || 0).toFixed(0)}N</b>
                    </div>
                    <div class="stat-mini-row">
                        <span>转速</span>
                        <b>${(s.rotational_speed || s.RotationalSpeed || 0).toFixed(1)}RPM</b>
                    </div>
                    <div class="stat-mini-row">
                        <span>油膜</span>
                        <b>${(s.oil_film_thickness || s.OilFilmThickness || 0).toFixed(2)}μm</b>
                    </div>
                </div>
            </div>`;
    },

    renderMaintenancePlan(plan) {
        const div = document.getElementById("maintenance-plan-panel");
        if (!div || !plan.recommended_actions || plan.recommended_actions.length === 0) return;

        const priorityLabels = { urgent: "🔴 紧急", high: "🟠 高", routine: "🔵 例行" };
        const priorityClasses = { urgent: "prio-urgent", high: "prio-high", routine: "prio-routine" };

        let html = `<h4>📋 智能维护建议</h4>`;
        plan.recommended_actions.forEach((a, i) => {
            html += `<div class="plan-action ${priorityClasses[a.priority] || ""}">
                <div class="action-header">
                    <span class="action-prio">${priorityLabels[a.priority] || a.priority}</span>
                    <span class="action-title">${a.title}</span>
                </div>
                <div class="action-detail">${a.detail}</div>`;
            if (a.recommended_lubricants && a.recommended_lubricants.length) {
                html += `<div class="action-sublist">
                    <b>推荐润滑剂:</b>
                    <ul>${a.recommended_lubricants.slice(0, 3).map(l =>
                        `<li>${l.name} - 减磨${l.wear_reduction.toFixed(0)}%</li>`).join("")}</ul>
                </div>`;
            }
            if (a.materials && a.materials.length) {
                html += `<div class="action-sublist">
                    <b>推荐材料:</b>
                    <ul>${a.materials.slice(0, 3).map(m =>
                        `<li>${m.name} - 耐磨系数${m.wear_factor.toFixed(2)}x</li>`).join("")}</ul>
                </div>`;
            }
            html += `</div>`;
        });

        div.innerHTML = html;
    },

    renderReplaceMaterials() {
        const select = document.getElementById("replacement-material");
        if (!select) return;

        const options = this.materials.map(m => {
            const lifeBonus = m.wear_resistance_factor >= 1
                ? ` (耐磨 +${((m.wear_resistance_factor - 1) * 100).toFixed(0)}%)`
                : ` (耐磨 ${(m.wear_resistance_factor * 100).toFixed(0)}%)`;
            return `<option value="${m.code}">${m.name_cn || m.name}${lifeBonus}</option>`;
        }).join("");
        select.innerHTML = options;
    },

    renderLubricantOptions() {
        const select = document.getElementById("lubricant-type");
        if (!select) return;

        const options = this.lubricants.map(l => {
            const reduction = ((l.wear_reduction_ratio || l.WearReductionRatio || 0) * 100).toFixed(0);
            return `<option value="${l.code}">${l.name_cn || l.name} (减磨 ${reduction}%)</option>`;
        }).join("");
        select.innerHTML = options;
    },

    async previewReplacement() {
        const code = document.getElementById("replacement-material")?.value;
        if (!code) return;

        const status = document.getElementById("replace-status");
        status.textContent = "⏳ 正在预测更换效果...";

        try {
            const preview = await API.previewBearingReplacement({
                bearing_id: this.bearingId,
                new_material_code: code,
                operator_name: this.operatorName,
                session_id: this.sessionId,
            });
            this.renderReplacementPreview(preview);
            status.textContent = "✅ 预测完成";
        } catch (e) {
            status.textContent = "❌ " + e.message;
        }
    },

    renderReplacementPreview(p) {
        const div = document.getElementById("replace-preview");
        if (!div || !p) return;

        const gain = p.life_extension_hours > 0;
        const pct = p.life_extension_percent;
        const yearsGain = p.life_extension_hours / 8760;

        div.innerHTML = `
            <div class="preview-card">
                <h4>🔧 更换效果预测</h4>
                <div class="preview-grid">
                    <div class="pv-item">
                        <div class="pv-label">当前磨损</div>
                        <div class="pv-value">${p.current_wear_um.toFixed(2)} μm</div>
                    </div>
                    <div class="pv-item">
                        <div class="pv-label">当前剩余寿命</div>
                        <div class="pv-value">${p.current_predicted_life_hours.toFixed(0)} h</div>
                    </div>
                    <div class="pv-item highlight">
                        <div class="pv-label">更换后寿命</div>
                        <div class="pv-value ${gain ? "good" : "bad"}">${p.projected_life_hours.toFixed(0)} h</div>
                    </div>
                    <div class="pv-item big-gain ${gain ? "gain" : "loss"}">
                        <div class="pv-label">寿命变化</div>
                        <div class="pv-value">${gain ? "+" : ""}${yearsGain.toFixed(2)} 年</div>
                        <div class="pv-sub">(${gain ? "+" : ""}${pct.toFixed(0)}%)</div>
                    </div>
                </div>
                <div class="pv-summary">
                    ${p.action_summary}
                    ${p.maintenance_cost_hint ? `<div class="cost-hint">💰 ${p.maintenance_cost_hint}</div>` : ""}
                </div>
            </div>`;
    },

    async executeReplacement() {
        const code = document.getElementById("replacement-material")?.value;
        if (!code) return;
        if (!confirm("确认执行轴承更换？这将模拟真实维护操作。")) return;

        const status = document.getElementById("replace-status");
        status.textContent = "🔧 正在执行更换...";

        try {
            const result = await API.executeBearingReplacement({
                bearing_id: this.bearingId,
                new_material_code: code,
                operator_name: this.operatorName || `游客(${this.sessionId.slice(-6)})`,
                session_id: this.sessionId,
                notes: "虚拟维护体验 - 前端模拟操作",
            });
            this.actionsTaken++;
            this.totalSavings += 2000;
            status.innerHTML = `🎉 更换成功！操作记录ID: <b>${result.record?.id || "-"}</b>`;
            this.updateExperienceStats();
            setTimeout(() => {
                this.loadBearingInfo();
                this.loadMaintenanceHistory();
            }, 1000);
        } catch (e) {
            status.textContent = "❌ 操作失败: " + e.message;
        }
    },

    async previewLubrication() {
        const code = document.getElementById("lubricant-type")?.value;
        const amountStr = document.getElementById("lubricant-amount")?.value || "200";
        const amount = parseFloat(amountStr) || 200;
        if (!code) return;

        const status = document.getElementById("lube-status");
        status.textContent = "⏳ 正在预测润滑效果...";

        try {
            const preview = await API.previewLubricantAddition({
                bearing_id: this.bearingId,
                lubricant_code: code,
                lubricant_amount_ml: amount,
                operator_name: this.operatorName,
                session_id: this.sessionId,
            });
            this.renderLubricationPreview(preview);
            status.textContent = "✅ 预测完成";
        } catch (e) {
            status.textContent = "❌ " + e.message;
        }
    },

    renderLubricationPreview(p) {
        const div = document.getElementById("lube-preview");
        if (!div || !p) return;

        const gain = p.life_extension_hours > 0;
        const yearsGain = p.life_extension_hours / 8760;

        div.innerHTML = `
            <div class="preview-card">
                <h4>🛢️ 添加润滑剂效果预测</h4>
                <div class="preview-grid">
                    <div class="pv-item">
                        <div class="pv-label">磨损率(更换前)</div>
                        <div class="pv-value">${p.old_wear_rate_um_per_hour.toFixed(4)} μm/h</div>
                    </div>
                    <div class="pv-item">
                        <div class="pv-label">当前剩余寿命</div>
                        <div class="pv-value">${p.current_predicted_life_hours.toFixed(0)} h</div>
                    </div>
                    <div class="pv-item highlight">
                        <div class="pv-label">改善后磨损率</div>
                        <div class="pv-value good">${p.new_wear_rate_um_per_hour.toFixed(4)} μm/h</div>
                    </div>
                    <div class="pv-item big-gain ${gain ? "gain" : "loss"}">
                        <div class="pv-label">寿命延长</div>
                        <div class="pv-value">${gain ? "+" : ""}${yearsGain.toFixed(2)} 年</div>
                        <div class="pv-sub">(${gain ? "+" : ""}${p.life_extension_percent.toFixed(0)}%)</div>
                    </div>
                </div>
                <div class="pv-summary">
                    ${p.action_summary}
                    ${p.maintenance_cost_hint ? `<div class="cost-hint">💰 ${p.maintenance_cost_hint}</div>` : ""}
                </div>
            </div>`;
    },

    async executeLubrication() {
        const code = document.getElementById("lubricant-type")?.value;
        const amountStr = document.getElementById("lubricant-amount")?.value || "200";
        const amount = parseFloat(amountStr) || 200;
        if (!code) return;
        if (!confirm("确认添加润滑剂？")) return;

        const status = document.getElementById("lube-status");
        status.textContent = "🛢️ 正在注入润滑剂...";

        try {
            const result = await API.executeLubricantAddition({
                bearing_id: this.bearingId,
                lubricant_code: code,
                lubricant_amount_ml: amount,
                operator_name: this.operatorName || `游客(${this.sessionId.slice(-6)})`,
                session_id: this.sessionId,
                notes: "虚拟维护体验 - 前端模拟操作",
            });
            this.actionsTaken++;
            this.totalSavings += 500;
            status.innerHTML = `✅ 润滑完成！操作记录ID: <b>${result.record?.id || "-"}</b>`;
            this.updateExperienceStats();
            setTimeout(() => {
                this.loadBearingInfo();
                this.loadMaintenanceHistory();
            }, 1000);
        } catch (e) {
            status.textContent = "❌ 操作失败: " + e.message;
        }
    },

    updateExperienceStats() {
        const counter = document.getElementById("exp-stats");
        if (!counter) return;
        counter.innerHTML = `
            <div class="exp-stat">
                <span class="exp-icon">🛠️</span>
                <span class="exp-num">${this.actionsTaken}</span>
                <span class="exp-label">维护操作</span>
            </div>
            <div class="exp-stat">
                <span class="exp-icon">💵</span>
                <span class="exp-num">${this.totalSavings.toLocaleString()}</span>
                <span class="exp-label">模拟节约(元)</span>
            </div>
            <div class="exp-stat">
                <span class="exp-icon">🆔</span>
                <span class="exp-num small">${this.sessionId}</span>
                <span class="exp-label">会话ID</span>
            </div>`;
    },

    renderHistory() {
        const div = document.getElementById("maintenance-history");
        if (!div) return;

        const actions = this.maintenanceHistory.filter(h =>
            !h.user_session_id || h.user_session_id === this.sessionId || this.maintenanceHistory.length < 20
        ).slice(0, 10);

        if (actions.length === 0) {
            div.innerHTML = `<div class="empty-history">暂无维护记录，开始您的虚拟维护之旅吧！</div>`;
            return;
        }

        const typeIcons = { replace_bearing: "🔧", add_lubricant: "🛢️", inspection: "🔍" };
        const typeLabels = { replace_bearing: "更换轴承", add_lubricant: "添加润滑剂", inspection: "巡检" };

        div.innerHTML = `<h4>📜 维护操作记录</h4>` + actions.map(h => {
            const when = new Date(h.performed_at || h.PerformedAt);
            const age = this.timeAgo(when);
            const wearBefore = h.wear_before_um || h.WearBeforeUm;
            const wearAfter = h.wear_after_um || h.WearAfterUm;
            const gain = wearAfter !== null && wearAfter !== undefined && wearBefore !== null && wearBefore !== undefined
                ? (((wearBefore - wearAfter) / Math.max(wearBefore, 1)) * 100).toFixed(0) + "%" : null;

            return `<div class="history-item">
                <div class="history-icon">${typeIcons[h.maintenance_type] || "📋"}</div>
                <div class="history-body">
                    <div class="history-title">
                        ${typeLabels[h.maintenance_type] || h.maintenance_type}
                        <span class="history-age">${age}</span>
                    </div>
                    <div class="history-action">${h.action || h.Action}</div>
                    <div class="history-meta">
                        <span>操作人: ${h.operator_name || "游客"}</span>
                        ${gain ? `<span class="gain-text">磨损降低 ${gain}</span>` : ""}
                    </div>
                </div>
            </div>`;
        }).join("");
    },

    timeAgo(date) {
        const ms = Date.now() - date.getTime();
        const secs = Math.floor(ms / 1000);
        if (secs < 60) return "刚刚";
        const mins = Math.floor(secs / 60);
        if (mins < 60) return `${mins}分钟前`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}小时前`;
        const days = Math.floor(hours / 24);
        return `${days}天前`;
    },
};
