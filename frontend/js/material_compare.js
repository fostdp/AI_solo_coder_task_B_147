const MaterialCompare = {
    materials: [],
    selectedMaterials: new Set(),
    result: null,
    bearingId: 1,
    chart: null,

    init(bearingId = 1) {
        this.bearingId = bearingId;
        this.loadMaterials();
        this.bindEvents();
    },

    async loadMaterials() {
        try {
            const res = await API.getMaterialsDetail();
            this.materials = res.data || [];
            this.renderMaterialList();
            this.selectedMaterials.add("wood_oak");
            this.selectedMaterials.add("wood_ironbark");
            this.selectedMaterials.add("bronze_ancient");
            this.selectedMaterials.add("cast_iron_ancient");
            this.renderSelection();
        } catch (e) {
            console.error("[MaterialCompare] 加载材料库失败:", e);
        }
    },

    bindEvents() {
        const btn = document.getElementById("btn-run-material-compare");
        if (btn) btn.onclick = () => this.runComparison();
    },

    renderMaterialList() {
        const container = document.getElementById("material-list-container");
        if (!container) return;

        const groups = {
            ancient_wood: { title: "[Wood] 古代木材", items: [] },
            ancient_metal: { title: "[Metal] 古代金属", items: [] },
            ancient_composite: { title: "[Composite] 古代复合材料", items: [] },
            modern: { title: "[Modern] 现代轴承材料", items: [] },
        };

        this.materials.forEach(m => {
            if (m.era === "ancient") {
                if (m.category === "wood") groups.ancient_wood.items.push(m);
                else if (m.category === "metal") groups.ancient_metal.items.push(m);
                else groups.ancient_composite.items.push(m);
            } else {
                groups.modern.items.push(m);
            }
        });

        let html = "";
        for (const [key, group] of Object.entries(groups)) {
            if (group.items.length === 0) continue;
            html += `<div class="material-group">
                <div class="group-title">${group.title}</div>
                <div class="material-grid">`;
            group.items.forEach(m => {
                const checked = this.selectedMaterials.has(m.code) ? "checked" : "";
                const wearIcon = m.wear_resistance_factor > 5 ? "*" : m.wear_resistance_factor > 1 ? "+" : "";
                html += `<label class="material-card ${checked ? "selected" : ""}" data-code="${m.code}">
                    <input type="checkbox" value="${m.code}" ${checked}/>
                    <div class="card-header">
                        <span class="mat-name">${m.name_cn || m.name}</span>
                        <span class="wear-badge">${wearIcon}</span>
                    </div>
                    <div class="card-meta">
                        <span>硬度 <b>${m.hardness_hv_nominal || m.hardnessHVNominal}</b> HV</span>
                        <span>耐磨系数 <b>${(m.wear_resistance_factor || 1).toFixed(2)}</b>x</span>
                    </div>
                </label>`;
            });
            html += `</div></div>`;
        }

        container.innerHTML = html;

        container.querySelectorAll(".material-card").forEach(card => {
            card.onclick = (e) => {
                if (e.target.tagName === "INPUT") return;
                const checkbox = card.querySelector("input");
                checkbox.checked = !checkbox.checked;
                this.toggleMaterial(card.dataset.code, checkbox.checked);
                card.classList.toggle("selected", checkbox.checked);
            };
            const input = card.querySelector("input");
            input.onchange = (e) => {
                this.toggleMaterial(e.target.value, e.target.checked);
                card.classList.toggle("selected", e.target.checked);
            };
        });
    },

    toggleMaterial(code, on) {
        if (on) this.selectedMaterials.add(code);
        else this.selectedMaterials.delete(code);
        this.renderSelection();
    },

    renderSelection() {
        const countEl = document.getElementById("material-selected-count");
        if (countEl) countEl.textContent = this.selectedMaterials.size;
    },

    async runComparison() {
        if (this.selectedMaterials.size < 2) {
            alert("请至少选择 2 种材料进行对比");
            return;
        }

        const status = document.getElementById("material-compare-status");
        if (status) status.textContent = "[Running] 正在运行仿真计算...";

        try {
            this.result = await API.compareMaterials({
                bearing_id: this.bearingId,
                material_codes: Array.from(this.selectedMaterials),
                simulation_hours: 8760 * 2,
                save_report: true,
                title: "材料对比 - " + new Date().toLocaleDateString(),
            });
            this.renderResults();
            if (status) status.textContent = "[OK] 计算完成";
        } catch (e) {
            if (status) status.textContent = "[Error] 计算失败: " + e.message;
            console.error(e);
        }
    },

    renderResults() {
        const resultsDiv = document.getElementById("material-compare-results");
        if (!resultsDiv || !this.result) return;

        const items = this.result.items || [];
        if (items.length === 0) {
            resultsDiv.innerHTML = '<p class="empty">无对比结果</p>';
            return;
        }

        const eraLabels = { ancient: "[Ancient] 古代", modern: "[Modern] 现代" };
        const bestLife = items[0].predicted_life_hours || items[0].PredictedLifeHours;

        let tableHtml = `
            <div class="compare-header">
                <h3>[Result] 材料磨损寿命对比分析</h3>
                <div class="compare-meta">
                    基准轴承ID: <b>${this.result.base_bearing_id || this.result.BaseBearingID}</b> |
                    工况: 载荷 <b>${this.result.reference_load_n || this.result.ReferenceLoad} N</b>,
                    转速 <b>${this.result.reference_speed_rpm || this.result.ReferenceSpeed} RPM</b>,
                    温度 <b>${this.result.reference_temp_celsius || this.result.ReferenceTemp} C</b>
                </div>
            </div>

            <div class="compare-table-wrap">
                <table class="compare-table">
                    <thead>
                        <tr>
                            <th>排名</th>
                            <th>材料名称</th>
                            <th>时代</th>
                            <th>硬度(HV)</th>
                            <th>累计磨损(um)</th>
                            <th>磨损率(um/h)</th>
                            <th>EHL lambda 参数</th>
                            <th>预估寿命</th>
                            <th>最佳寿命比</th>
                        </tr>
                    </thead>
                    <tbody>`;

        items.forEach((item, i) => {
            const name = item.material_name || item.MaterialName;
            const life = item.predicted_life_hours || item.PredictedLifeHours;
            const lifeYears = item.predicted_life_years || item.PredictedLifeYears;
            const ratio = item.life_ratio_vs_best || item.LifeRatioVsBest;
            const barWidth = Math.min(100, (ratio * 100)).toFixed(0);
            const rowClass = i === 0 ? "best-row" : (i === items.length - 1 ? "worst-row" : "");

            tableHtml += `<tr class="${rowClass}">
                <td><span class="rank-badge rank-${i + 1}">${i + 1}</span></td>
                <td class="mat-cell"><b>${name}</b><div class="mat-note">${this.shorten(item.historical_note || item.HistoricalNote, 50)}</div></td>
                <td>${eraLabels[item.era || item.Era] || item.era || item.Era}</td>
                <td>${item.hardness_hv || item.HardnessHV}</td>
                <td>${(item.total_wear_microm || item.TotalWearMicrom).toFixed(2)}</td>
                <td>${(item.wear_rate_um_per_hour || item.WearRateUmPerHour).toFixed(4)}</td>
                <td>${(item.ehl_mean_lambda || item.EHLMeanLambda).toFixed(2)}</td>
                <td><b>${life.toFixed(0)} h</b><br><span class="sub">(${lifeYears.toFixed(1)} 年)</span></td>
                <td>
                    <div class="life-bar-wrap">
                        <div class="life-bar" style="width:${barWidth}%"></div>
                        <span>${(ratio * 100).toFixed(0)}%</span>
                    </div>
                </td>
            </tr>`;
        });

        tableHtml += `</tbody></table></div>`;

        const best = items[0];
        const worst = items[items.length - 1];
        const improvement = best.predicted_life_hours / Math.max(worst.predicted_life_hours, 1);

        tableHtml += `
            <div class="compare-insights">
                <h4>[Insight] 分析结论</h4>
                <ul>
                    <li>最优材料：<b>${best.material_name || best.MaterialName}</b>，预估寿命 <b>${(best.predicted_life_years || best.PredictedLifeYears).toFixed(1)}年</b></li>
                    <li>磨损率差异：最好 vs 最差 = <b>${improvement.toFixed(1)} 倍</b>，材料选择至关重要！</li>
                    <li>耐磨性主要与 <b>硬度</b>、<b>表面粗糙度</b> 正相关，EHL 润滑状态影响磨损系数</li>
                </ul>
            </div>`;

        resultsDiv.innerHTML = tableHtml;
        this.renderChart(items);
    },

    renderChart(items) {
        const ctx = document.getElementById("material-compare-chart");
        if (!ctx || !window.Chart) return;
        if (this.chart) this.chart.destroy();

        this.chart = new Chart(ctx, {
            type: "bar",
            data: {
                labels: items.map(i => (i.material_name || i.MaterialName || "").substring(0, 8)),
                datasets: [
                    {
                        label: "预估寿命 (千小时)",
                        data: items.map(i => ((i.predicted_life_hours || i.PredictedLifeHours) / 1000).toFixed(1)),
                        backgroundColor: "rgba(79, 195, 247, 0.7)",
                        borderColor: "rgba(79, 195, 247, 1)",
                        borderWidth: 1,
                        yAxisID: "y",
                    },
                    {
                        label: "磨损率 (um/h, 反向)",
                        data: items.map(i => 1 / Math.max(i.wear_rate_um_per_hour || i.WearRateUmPerHour, 0.0001)),
                        type: "line",
                        borderColor: "#ffb74d",
                        backgroundColor: "rgba(255, 183, 77, 0.2)",
                        tension: 0.3,
                        yAxisID: "y1",
                    },
                ],
            },
            options: {
                responsive: true,
                scales: {
                    y: { beginAtZero: true, position: "left", title: { display: true, text: "寿命 (kh)" } },
                    y1: { position: "right", grid: { drawOnChartArea: false } },
                },
                plugins: {
                    legend: { position: "top" },
                    title: { display: true, text: "材料寿命对比柱状图" },
                },
            },
        });
    },

    shorten(str, maxLen) {
        if (!str) return "";
        return str.length > maxLen ? str.substring(0, maxLen) + "..." : str;
    },
};
