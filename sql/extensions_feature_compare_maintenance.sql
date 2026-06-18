-- ========================================
-- 新功能扩展表：材料对比、润滑剂、虚拟维护
-- 执行时间：在 init.sql 之后执行
-- ========================================

BEGIN;

-- 1. 材料参数参考表（配置同步，数据库也存一份便于查询）
CREATE TABLE IF NOT EXISTS bearing_materials_ref (
    material_code       VARCHAR(50) PRIMARY KEY,
    name_cn             VARCHAR(200) NOT NULL,
    era                 VARCHAR(20) NOT NULL,
    category            VARCHAR(20) NOT NULL,
    hardness_hv_nominal NUMERIC(10,2) NOT NULL,
    elastic_modulus_gpa NUMERIC(10,2),
    density_kg_m3       INTEGER,
    wear_resistance_factor NUMERIC(5,3),
    historical_note     TEXT,
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

-- 2. 润滑剂参考表
CREATE TABLE IF NOT EXISTS lubricants_ref (
    lubricant_code          VARCHAR(50) PRIMARY KEY,
    name_cn                 VARCHAR(200) NOT NULL,
    category                VARCHAR(20) NOT NULL,
    era                     VARCHAR(20) NOT NULL,
    viscosity_40c_cst       NUMERIC(10,2) NOT NULL,
    viscosity_index         INTEGER,
    wear_reduction_ratio    NUMERIC(5,3),
    lubrication_effect      NUMERIC(5,3),
    max_life_hours          INTEGER,
    historical_note         TEXT,
    created_at              TIMESTAMPTZ DEFAULT NOW()
);

-- 3. 维护记录表（超表，时间序列记录所有维护操作）
CREATE TABLE IF NOT EXISTS maintenance_records (
    id                  BIGSERIAL,
    bearing_id          INTEGER NOT NULL REFERENCES bearings(id) ON DELETE CASCADE,
    performed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    maintenance_type    VARCHAR(30) NOT NULL,
    action              VARCHAR(200) NOT NULL,
    old_material_code   VARCHAR(50),
    new_material_code   VARCHAR(50),
    lubricant_code      VARCHAR(50),
    lubricant_amount_ml NUMERIC(10,2),
    wear_before_um      NUMERIC(12,6),
    wear_after_um       NUMERIC(12,6),
    operator_name       VARCHAR(100),
    notes               TEXT,
    user_session_id     VARCHAR(100),
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('maintenance_records', 'performed_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_maintenance_bearing ON maintenance_records(bearing_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_maintenance_session ON maintenance_records(user_session_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_type ON maintenance_records(maintenance_type);

-- 4. 对比分析任务表（记录用户发起的对比分析任务）
CREATE TABLE IF NOT EXISTS comparison_reports (
    id                  BIGSERIAL PRIMARY KEY,
    report_type         VARCHAR(30) NOT NULL,
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    base_bearing_id     INTEGER REFERENCES bearings(id) ON DELETE SET NULL,
    parameters_json     JSONB NOT NULL,
    result_json         JSONB NOT NULL,
    user_session_id     VARCHAR(100),
    title               VARCHAR(300)
);

CREATE INDEX IF NOT EXISTS idx_comparison_type ON comparison_reports(report_type, generated_at DESC);
CREATE INDEX IF NOT EXISTS idx_comparison_session ON comparison_reports(user_session_id);

-- 5. 润滑剂使用状态表（记录轴承当前润滑状态）
CREATE TABLE IF NOT EXISTS bearing_lubrication_status (
    bearing_id          INTEGER PRIMARY KEY REFERENCES bearings(id) ON DELETE CASCADE,
    lubricant_code      VARCHAR(50) NOT NULL,
    last_applied_at     TIMESTAMPTZ NOT NULL,
    applied_amount_ml   NUMERIC(10,2),
    hours_since_lube    NUMERIC(12,4),
    degradation_ratio   NUMERIC(5,3),
    next_recommended_at TIMESTAMPTZ,
    applied_by          VARCHAR(100),
    notes               TEXT,
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

-- ========================================
-- 初始化材料和润滑剂参考数据
-- ========================================

-- 插入古代轴承材料
INSERT INTO bearing_materials_ref (material_code, name_cn, era, category, hardness_hv_nominal, elastic_modulus_gpa, density_kg_m3, wear_resistance_factor, historical_note) VALUES
('bronze_ancient', '古代青铜 (锡青铜 CuSn10)', 'ancient', 'metal', 110, 100, 8800, 0.85, '中国商周时期即掌握青铜冶铸，汉代已广泛用于轴瓦。耐腐蚀且有自润滑性。'),
('cast_iron_ancient', '古代铸铁 (灰口铸铁)', 'ancient', 'metal', 220, 120, 7200, 0.6, '春秋时期出现生铁冶铸，铸铁件耐磨但性脆，石墨片有一定减磨作用。'),
('wood_oak', '橡木 (硬木)', 'ancient', 'wood', 22, 12, 750, 0.35, '橡木、枣木等硬木是最古老的轴承材料，加工简单但易磨损、怕水。常包铜铁皮增强。'),
('wood_ironbark', '青冈木 (铁栎)', 'ancient', 'wood', 38, 18, 1100, 0.5, '西南地区常用的铁栎，密度极高、耐腐耐磨，是筒车轴承的优选。配合桐油润滑可达数年寿命。'),
('wood_wrapped_copper', '包铜铁皮木轴瓦', 'ancient', 'composite', 90, 60, 3200, 1.2, '明清常见工艺：木胎外包铜皮或铁皮，既利用木的减震性，又有金属的耐磨性。'),
('modern_ball_bearing', '现代深沟球轴承 (GCr15钢制)', 'modern', 'rolling', 750, 207, 7850, 20.0, '20世纪精密工业的结晶。点接触赫兹应力极高，但通过滚变滑动可将摩擦系数降至0.001~0.005。'),
('modern_roller_bearing', '现代调心滚子轴承', 'modern', 'rolling', 720, 207, 7850, 15.0, '线接触可承受大径向载荷和冲击，承载能力是球轴承的2~3倍。'),
('modern_bushing_babbit', '现代巴氏合金滑动轴承', 'modern', 'metal', 28, 35, 9300, 5.0, '锡基巴氏合金是现代滑动轴承首选，嵌藏性、顺应性极佳。')
ON CONFLICT (material_code) DO NOTHING;

-- 插入润滑剂参考数据
INSERT INTO lubricants_ref (lubricant_code, name_cn, category, era, viscosity_40c_cst, viscosity_index, wear_reduction_ratio, lubrication_effect, max_life_hours, historical_note) VALUES
('vegetable_tung', '桐油 (植物油)', 'vegetable', 'ancient', 280, 195, 0.42, 0.55, 1200, '中国特有，从油桐籽榨取。干燥快、附着力强，古代最重要的工业油料。宋代已广泛用于车船润滑。'),
('vegetable_rape', '菜籽油 (植物油)', 'vegetable', 'ancient', 38, 220, 0.35, 0.48, 800, '油菜花籽榨取，易得且便宜。粘度较低，适合轻型低速机械。江南农家常用。'),
('vegetable_sesame', '芝麻油 (植物油)', 'vegetable', 'ancient', 35, 200, 0.48, 0.6, 1500, '古称胡麻，汉代从西域传入。稳定性好、不易酸败，是古代高级润滑剂。宫廷精密仪器多使用。'),
('animal_lard', '猪油 (动物油)', 'animal', 'ancient', 45, 165, 0.52, 0.65, 500, '易得的动物油脂，常温半固体，承载能力强但易酸败。冬季需加热融化使用。'),
('animal_beef_tallow', '牛油 (动物油)', 'animal', 'ancient', 65, 145, 0.55, 0.68, 600, '熔点高，适合重型高温工况。宋代《武经总要》记载用于军事器械。牧区大量使用。'),
('animal_whale', '鲸油 (动物油)', 'animal', 'ancient_rare', 25, 180, 0.5, 0.72, 3000, '极其珍贵的古代高级润滑油，低温流动性极佳，抗氧化能力强。明清海贸时代进口。'),
('mineral_paraffin', '石蜡基矿物油 (现代)', 'mineral', 'modern', 68, 95, 0.78, 0.85, 8000, '20世纪石油工业产物，性能稳定、成本低。添加ZDDP等抗磨剂后性能远超天然油脂。'),
('mineral_synthetic_pao', 'PAO合成油 (现代)', 'synthetic', 'modern', 68, 145, 0.9, 0.95, 20000, '1970年代航天科技产物，人工合成的聚α烯烃。宽温域、超长寿命，现代高端机械首选。'),
('mineral_modern_additive', '现代极压齿轮油', 'mineral', 'modern', 220, 90, 0.93, 0.92, 12000, '添加硫磷型极压添加剂(EP)，可承受重载冲击工况。')
ON CONFLICT (lubricant_code) DO NOTHING;

-- ========================================
-- 视图：轴承维护历史汇总
-- ========================================
CREATE OR REPLACE VIEW v_bearing_maintenance_summary AS
SELECT
    b.id AS bearing_id,
    b.bearing_code,
    b.position,
    COUNT(m.id) AS total_maintenance_count,
    COUNT(CASE WHEN m.maintenance_type = 'replace_bearing' THEN 1 END) AS replacement_count,
    COUNT(CASE WHEN m.maintenance_type = 'add_lubricant' THEN 1 END) AS lubrication_count,
    MAX(m.performed_at) AS last_maintenance_at,
    MAX(CASE WHEN m.maintenance_type = 'replace_bearing' THEN m.performed_at END) AS last_replacement_at,
    MAX(CASE WHEN m.maintenance_type = 'add_lubricant' THEN m.performed_at END) AS last_lubrication_at,
    SUM(COALESCE(m.lubricant_amount_ml, 0)) AS total_lubricant_used_ml
FROM bearings b
LEFT JOIN maintenance_records m ON m.bearing_id = b.id
GROUP BY b.id, b.bearing_code, b.position;

COMMENT ON VIEW v_bearing_maintenance_summary IS '轴承维护历史汇总视图';

COMMIT;
