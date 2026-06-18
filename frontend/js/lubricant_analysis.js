const LubricantAnalysis = {
    lubricants: [],
    materials: [],
    selectedLubricants: new Set(),
    selectedMaterialCode: "bronze_ancient",
    result: null,
    chart: null,
    bearingId: 1,

    init(bearingId = 1) {
        this.bearingId = bearingId;
        this.loadReferenceData();
        this.bindEvents();
    },

    async loadReferenceData() {
        try {
            const [lubRes, matRes] = await Promise.all([
                API.getLubricantsDetail(),
                API.getMaterials(),
            ]);
            this.lubricants = lubRes.data || [];
            this.materials = matRes.data || [];
            this.renderMaterialSelect();
            this.renderLubricantList();

            this.selectedLubricants.add("vegetable_tung");
            this.selectedLubricants.add("vegetable_rape");
            this.selectedLubricants.add("vegetable_sesame");
            this.selectedLubricants.add("animal_lard");
            this.selectedLubricants.add("animal_beef_tallow");
            this.renderSelection();
            this.updateCardSelection();
        } catch (e) {
            console.error("加载润滑剂数据失败:", e);
        }
    },

    bindEvents() {
        const btn = document.getElementById("btn-run-lubricant-compare");
        if (btn) btn.onclick = () => this.runComparison();
    },

    renderMaterialSelect() {
        const select = document.getElementById("base-material-select");
        if (!select) return;

        const ancientMats = this.materials.filter(m =>
            m.era === "ancient" || m.category === "wood" || m.category === "metal" || m.category === "composite"
        );

        select.innerHTML = ancientMats.map(m =>
            `<option value="${m.code}" ${m.code === this.selectedMaterialCode ? "selected" : ""}>${m.name_cn || m.name} (${m.hardness_hv_nominal || m.hardnessHVNominal}HV)</option>`
        ).join("");

        select.onchange = (e) => {
            this.selectedMaterialCode = e.target.value;
        };
    },

    renderLubricantList() {
        const container = document.getElementById("lubricant-list-container");
        if (!container) return;

        const groups = {
            vegetable: { title: "🌿 植物油 (古代主流)", items: [] },
            animal: { title: "🐄 动物油脂 (古代高端)", items: [] },
            mineral: { title: "🛢️ 矿物油 (现代工业)", items: [] },
            synthetic: { title: "🧪 合成油 (现代尖端)", items: [] },
        };

        this.lubricants.forEach(l => {
            const cat = l.category || l.Category;
            if (groups[cat]) groups[cat].items.push(l);
        });

        let html = "";
        for (const [key, group] of Object.entries(groups)) {
            if (group.items.length === 0) continue;
            html += `<div class="lubricant-group">
                <div class="group-title">${group.title}</div>
                <div class="lubricant-grid">`;
            group.items.forEach(l => {
                const checked = this.selectedLubricants.has(l.code) ? "checked" : "";
                const reduction = (l.wear_reduction_ratio || l.WearReductionRatio || 0) * 100;
                const icon = key === "vegetable" ? "🌿" : key === "animal" ? "🐄" : key === "mineral" ? "🛢️" : "🧪";
                html += `<label class="lubricant-card ${checked ? "selected" : ""}" data-code="${l.code}">
                    <input type="checkbox" value="${l.code}" ${checked}/>
                    <div class="card-header">
                        <span class="lube-icon">${icon}</span>
                        <span class="lube-name">${l.name_cn || l.name}</span>
                    </div>
                    <div class="card-meta">
                        <span>粘度 <b>${l.viscosity_40c_cst || l.Viscosity40C || l.viscosity_40c_mm2_per_s}</b> cSt</span>
                        <span>减磨 <b>${reduction.toFixed(0)}</b>%</span>
                    </div>
                </label>`;
            });
            html += `</div></div>`;
        }

        container.innerHTML = html;

        container.querySelectorAll(".lubricant-card").forEach(card => {
            card.onclick = (e) => {
                if (e.target.tagName === "INPUT") return;
                const checkbox = card.querySelector("input");
                checkbox.checked = !checkbox.checked;
                this.toggleLubricant(card.dataset.code, checkbox.checked);
                card.classList.toggle("selected", checkbox.checked);
            };
            const input = card.querySelector("input");
            input.onchange = (e) => {
                this.toggleLubricant(e.target.value, e.target.checked);
                card.classList.toggle("selected", e.target.checked);
            };
        });
    },

    updateCardSelection() {
        document.querySelectorAll(".lubricant-card").forEach(card => {
            const code = card.dataset.code;
            card.classList.toggle("selected", this.selectedLubricants.has(code));
            const cb = card.querySelector("input");
            if (cb) cb.checked = this.selectedLubricants.has(code);
        });
    },

    toggleLubricant(code, on) {
        if (on) this.selectedLubricants.add(code);
        else this.selectedLubricants.delete(code);
        this.renderSelection();
    },

    renderSelection() {
        const countEl = document.getElementById("lubricant-selected-count");
        if (countEl) countEl.textContent = this.selectedLubricants.size;
    },

    async runComparison() {
        if (this.selectedLubricants.size < 2) {
            alert("请至少选择 2 种润滑剂进行对比");
            return;
        }

        const status = document.getElementById("lubricant-compare-status");
        status.textContent = "⏳ 正在运行仿真计算...";

        try {
            this.result = await API.compareLubricants({
                bearing_id: this.bearingId,
                material_code: this.selectedMaterialCode,
                lubricant_codes: Array.from(this.selectedLubricants),
                simulation_hours: 8760 * 2,
                save_report: true,
                title: "润滑剂对比 - " + new Date().toLocaleDateString(),
            });
            this.renderResults();
            status.textContent = "✅ 计算完成";
        } catch (e) {
            status.textContent = "❌ 计算失败: " + e.message;
            console.error(e);
        }
    },

    renderResults() {
        const div = document.getElementById("lubricant-compare-results");
        if (!div || !this.result) return;

        const items = this.result.items || [];
        if (items.length === 0) {
            div.innerHTML = '<p class="empty">无对比结果</p>';
            return;
        }

        const catLabels = { vegetable: "🌿 植物", animal: "🐄 动物", mineral: "🛢️ 矿物", synthetic: "🧪 合成" };
        const bestLife = items[0].predicted_life_hours || items[0].PredictedLifeHours;

        let html = `
            <div class="compare-header">
                <h3>🧴 润滑剂磨损寿命影响分析</h3>
                <div class="compare-meta">
                    基础材料: <b>${this.result.base_material_code || this.result.BaseMaterial}</b> |
                    工况: 载荷 <b>${this.result.reference_load_n || this.result.ReferenceLoad} N</b>,
                    转速 <b>${this.result.reference_speed_rpm || this.result.ReferenceSpeed} RPM</b>
                </div>
            </div>

            <div class="compare-table-wrap">
                <table class="compare-table">
                    <thead>
                        <tr>
                            <th>排名</th>
                            <th>润滑剂</th>
                            <th>类别</th>
                            <th>粘度(cSt)</th>
                            <th>润滑状态</th>
                            <th>磨损率(μm/h)</th>
                            <th>较干磨减磨</th>
                            <th>寿命提升</th>
                            <th>预估寿命</th>
                            <th>推荐周期</th>
                        </tr>
                    </thead>
                    <tbody>`;

        items.forEach((item, i) => {
            const life = item.predicted_life_hours || item.PredictedLifeHours;
            const lifeYears = item.predicted_life_years || item.PredictedLifeYears;
            const reduction = item.wear_reduction_vs_dry_pct || item.WearReductionVsDry;
            const extension = item.life_extension_vs_dry_pct || item.LifeExtensionVsDry;
            const freq = item.recommended_frequency_hours || item.RecommendedFreqHours;
            const ratio = item.life_ratio_vs_best || item.LifeRatioVsBest;
            const barWidth = Math.min(100, (ratio * 100)).toFixed(0);
            const rowClass = i === 0 ? "best-row" : (i === items.length - 1 ? "worst-row" : "");

            const regimeBadge = (item.lubrication_regime || item.LubricationRegime || "-").replace(
                /全膜弹流润滑|混合润滑|边界润滑/g,
                s => s === "全膜弹流润滑" ? "🟢 全膜" : s === "混合润滑" ? "🟡 混合" : "🔴 边界"
            );

            html += `<tr class="${rowClass}">
                <td><span class="rank-badge rank-${i + 1}">${i + 1}</span></td>
                <td class="mat-cell"><b>${item.lubricant_name || item.LubricantName}</b>
                    <div class="mat-note">${this.shorten(item.historical_note || item.HistoricalNote, 45)}</div>
                </td>
                <td>${catLabels[item.category || item.Category] || item.category || item.Category}</td>
                <td>${item.viscosity_40c || item.ViscosityAt40C}</td>
                <td>${regimeBadge}</td>
                <td>${(item.wear_rate_um_per_hour || item.WearRateUmPerHour).toFixed(4)}</td>
                <td><span class="good">${reduction.toFixed(0)}%</span></td>
                <td><span class="good">+${extension.toFixed(0)}%</span></td>
                <td>
                    <div class="life-bar-wrap">
                        <div class="life-bar" style="width:${barWidth}%"></div>
                        <span>${lifeYears.toFixed(1)}年</span>
                    </div>
                </td>
                <td>${freq < 500 ? `${freq.toFixed(0)}h (${(freq/24).toFixed(0)}天)` : `${(freq/24/30).toFixed(1)}月`}</td>
            </tr>`;
        });

        html += `</tbody></table></div>`;

        const best = items[0];
        const worst = items[items.length - 1];
        const improvement = best.predicted_life_hours / Math.max(worst.predicted_life_hours, 1);
        const baseMaterial = this.result.base_material_code || this.result.BaseMaterial;

        html += `
            <div class="compare-insights">
                <h4>💡 分析结论</h4>
                <ul>
                    <li>最优润滑剂：<b>${best.lubricant_name || best.LubricantName}</b>，配合 <b>${baseMaterial}</b> 预估寿命 <b>${(best.predicted_life_years || best.PredictedLifeYears).toFixed(1)}年</b></li>
                    <li>润滑剂寿命差异可达 <b>${improvement.toFixed(1)} 倍</b>，古代高级润滑剂（芝麻油、鲸油）性能接近现代低端矿物油</li>
                    <li>润滑状态（EHL λ值）直接决定磨损系数：全膜磨损系数仅为边界的1/50</li>
                    <li>古代油脂问题是 <b>易酸败失效</b>，需频繁补加（3-7天/次），现代油可达数月甚至数年</li>
                </ul>
            </div>`;

        div.innerHTML = html;
        this.renderChart(items);
    },

    renderChart(items) {
        const ctx = document.getElementById("lubricant-compare-chart");
        if (!ctx) return;
        if (this.chart) this.chart.destroy();

        this.chart = new Chart(ctx, {
            type: "bar",
            data: {
                labels: items.map(i => (i.lubricant_name || i.LubricantName || "").substring(0, 8)),
                datasets: [
                    {
                        label: "预估寿命 (年)",
                        data: items.map(i => (i.predicted_life_years || i.PredictedLifeYears).toFixed(1)),
                        backgroundColor: "rgba(129, 212, 250, 0.7)",
                        borderColor: "#81d4fa",
                        borderWidth: 1,
                    },
                    {
                        label: "较干磨寿命提升 (%)",
                        data: items.map(i => (i.life_extension_vs_dry_pct || i.LifeExtensionVsDry).toFixed(0)),
                        type: "line",
                        borderColor: "#66bb6a",
                        backgroundColor: "rgba(102, 187, 106, 0.2)",
                        fill: true,
                        tension: 0.3,
                        yAxisID: "y1",
                    },
                ],
            },
            options: {
                responsive: true,
                scales: {
                    y: { beginAtZero: true, title: { display: true, text: "寿命 (年)" } },
                    y1: { position: "right", grid: { drawOnChartArea: false }, title: { display: true, text: "提升 (%)" } },
                },
                plugins: {
                    legend: { position: "top" },
                    title: { display: true, text: "润滑剂性能对比" },
                },
            },
        });
    },

    shorten(str, maxLen) {
        if (!str) return "";
        return str.length > maxLen ? str.substring(0, maxLen) + "..." : str;
    },
};
