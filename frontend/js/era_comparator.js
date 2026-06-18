const EraComparator = {
    result: null,
    chart: null,

    init() {
        this.bindEvents();
    },

    bindEvents() {
        const btnCross = document.getElementById("btn-run-cross-era");
        if (btnCross) btnCross.onclick = () => this.runCrossEra();
    },

    async runCrossEra() {
        const status = document.getElementById("cross-era-status");
        if (status) status.textContent = "[Running] 正在运行跨时代磨损对比...";

        try {
            const res = await API.crossEraComparison({
                bearing_diameter_mm: 150,
                bearing_width_mm: 80,
                reference_load_n: 5000,
                reference_speed_rpm: 15,
                simulation_hours: 8760 * 3,
                save_report: true,
            });
            this.renderCrossEra(res);
            if (status) status.textContent = "[OK] 跨时代对比完成";
        } catch (e) {
            if (status) status.textContent = "[Error] 对比失败: " + e.message;
            console.error(e);
        }
    },

    renderCrossEra(res) {
        const div = document.getElementById("cross-era-results");
        if (!div) return;

        const ancient = res.ancient_best || res.AncientBest;
        const modern = res.modern_best || res.ModernBest;
        const items = res.all_items || res.AllItems || [];
        const improvement = res.life_improvement_x || res.LifeImprovementX || 0;
        const reduction = res.wear_reduction_percent || res.WearReductionPct || 0;

        let html = `
            <div class="cross-era-wrap">
                <div class="era-card ancient">
                    <h3>[Ancient] 古代最优方案</h3>
                    <div class="era-main-metric">${(ancient?.predicted_life_years || ancient?.PredictedLifeYears || 0).toFixed(1)} <span class="unit">年</span></div>
                    <div class="era-name">${ancient?.material_name || ancient?.MaterialName || "-"}</div>
                    <div class="era-desc">磨损率: ${(ancient?.wear_rate_um_per_hour || ancient?.WearRateUmPerHour || 0).toFixed(4)} μm/h</div>
                </div>

                <div class="vs-arrow">
                    <div class="vs-text">[VS]</div>
                    <div class="improvement">
                        <div class="big-improve">${improvement.toFixed(0)}<span class="unit">x</span></div>
                        <div class="improv-label">现代寿命提升</div>
                    </div>
                </div>

                <div class="era-card modern">
                    <h3>[Modern] 现代最优方案</h3>
                    <div class="era-main-metric">${(modern?.predicted_life_years || modern?.PredictedLifeYears || 0).toFixed(1)} <span class="unit">年</span></div>
                    <div class="era-name">${modern?.material_name || modern?.MaterialName || "-"}</div>
                    <div class="era-desc">磨损率: ${(modern?.wear_rate_um_per_hour || modern?.WearRateUmPerHour || 0).toFixed(4)} μm/h</div>
                </div>
            </div>

            <div class="cross-stats">
                <div class="stat-card">
                    <div class="stat-value">${reduction.toFixed(1)}%</div>
                    <div class="stat-label">磨损率降低</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${items.length}</div>
                    <div class="stat-label">参与对比材料</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${(res.bearing_diameter || res.BearingDiameter)}</div>
                    <div class="stat-label">轴承直径 (mm)</div>
                </div>
            </div>`;

        const insights = res.insight_summary || res.InsightSummary || [];
        if (insights.length) {
            html += `<div class="compare-insights">
                <h4>[Insight] 跨时代洞察</h4>
                <ul>${insights.map(s => `<li>${s}</li>`).join("")}</ul>
            </div>`;
        }

        div.innerHTML = html;
        this.renderChart(items);
    },

    renderChart(items) {
        const canvas = document.getElementById("cross-era-chart");
        if (!canvas || !window.Chart) return;
        if (this.chart) this.chart.destroy();

        const labels = items.map(i => i.material_name || i.MaterialName);
        const lifeHours = items.map(i => i.predicted_life_hours || i.PredictedLifeHours);
        const eraColors = items.map(i => (i.era === "modern" ? "#2563eb" : "#d97706"));

        this.chart = new Chart(canvas, {
            type: "bar",
            data: {
                labels: labels,
                datasets: [{
                    label: "预估寿命 (小时)",
                    data: lifeHours,
                    backgroundColor: eraColors,
                }],
            },
            options: {
                indexAxis: "y",
                responsive: true,
                plugins: {
                    legend: { position: "top" },
                    title: { display: true, text: "跨时代材料寿命对比" },
                },
            },
        });
    },
};
