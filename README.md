# 古代水转筒车轴承磨损仿真与寿命预测系统

Ancient Noria Wheel Bearing Wear Simulation & Life Prediction System

某水利史团队对唐代水转筒车进行复原研究，对筒车滑动轴承进行长期磨损监测与寿命预测的全栈应用系统。

---

## 目录

- [系统架构](#系统架构)
- [模块说明](#模块说明)
- [新增功能（Feature）](#新增功能feature)
- [快速部署](#快速部署)
- [服务端口](#服务端口)
- [传感器模拟器](#传感器模拟器)
- [监控与运维](#监控与运维)
- [API 接口](#api-接口)
- [目录结构](#目录结构)

---

## 系统架构

### 整体架构图

```
                              ┌──────────────────────────────────────────────────────────────┐
                              │                        客户端                        │
                              │                    (浏览器 / Nginx :80                     │
                              │  ┌──────────────┐   ┌──────────────┐                │
                              │  │  3D 筒车模型 │   │ 轴承寿命仪表盘 │                │
                              │  │ waterwheel_3d  │   │ bearing_panel│                │
                              │  └──────────────┘   └──────────────┘                │
                              └───────────────────┬──────────────────────────────────────────┘
                                                  │ HTTP / WebSocket
                                                  ▼
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                              Go 后端服务 (Channel 消息驱动架构)                        │
│                                                                                     │
│  ┌─────────────────┐     SensorDataChan      ┌──────────────────┐                         │
│  │ modbus_     │ ─────────────────► │                  │  WearRequestChan             │
│  │ receiver     │                     │                  │ ──────────────────► ┌───────────┐   │
│  │ (数据采集校验) │                     │    Scheduler     │                     │ wear_     │   │
│  └─────────────────┘                     │   (调度器)        │ ◄────────────────── │ simulator │   │
│                                            │                  │    WearResultChan │           │   │
│  ┌─────────────────┐                     │                  │                     │ (Archard+EHL) │   │
│  │  REST API      │ ◄───────────────────┤                  │                     └───────────┘   │
│  │  (Gin :8080)  │                     │                  │                         │
│  └─────────────────┘                     │                  │  LifeRequestChan             │
│                                            │                  │ ──────────────────► ┌──────────┐ │
│  ┌─────────────────┐                     │                  │                     │ life_     │ │
│  │  Prometheus   │ :9090 /metrics       │                  │ ◄────────────────── │ predictor│ │
│  │  & pprof     │                     │                  │   LifeResultChan │            │ (Weibull)  │ │
│  │  :6060         │                     │                  │                     └──────────┘ │
│  └─────────────────┘                     │                  │                         │
│                                            │                  │   AlertChan                 │
│                                            │                  │ ──────────────────► ┌──────────┐ │
│                                            │                  │                     │ alarm_    │ │
│                                            │                  │                     │ mqtt     │ │
│                                            └──────────────────┘                     │ (MQTT推送) │ │
│                                                                                     └──────────┘ │
└──────────────────────────────────────────────┬─────────────────────────────────────────────────────────────────┘
                                       │
                ┌──────────────────────┼──────────────────────┐
                ▼                      ▼                      ▼
┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│    TimescaleDB      │  │   MQTT Broker        │  │   MQTT 订阅者(运维系统)  │
│    :5432           │  │   Mosquitto :1883     │  │                      │
│  · 原始数据 90天    │  │                      │  │ 告警主题：            │
│  · 5min聚合 30天   │  │  Topic:              │  │ noria/alerts/<id>/<type>
│  · 1h聚合 2年     │  │  noria/alerts/...   │  │                      │
│  · 1d聚合 10年    │  │                      │  │                      │
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘
```

### 数据流时序图

```
模拟器(Modbus TCP 5020)
    │
    ▼
modbus_receiver  →  validateSensorData()  →  SensorDataChan
                                               │
                                               ▼
                                         Scheduler
                                               │
                         ┌─────────────────────┼─────────────────────┐
                         ▼                     ▼                     ▼
                 WearRequestChan      LifeRequestChan       AlertChan
                         │                     │                     │
                         ▼                     ▼                     ▼
                 wear_simulator      life_predictor       alarm_mqtt
                 (Archard+EHL)       (Weibull+Bayes)        (MQTT publish)
                         │                     │                     │
                         ▼                     ▼                     ▼
                 WearResultChan       LifeResultChan        MQTT Broker
                         │                     │
                         └──────────┬──────────┘
                                    ▼
                              TimescaleDB
                           (磨损结果入库)
```

---

## 模块说明

### Go 后端模块（Channel 消息驱动）

| 模块 | 文件 | 职责 |
|------|------|------|
| **modbus_receiver** | `backend/internal/modules/modbus_receiver/receiver.go` | Modbus TCP 数据接收、物理量范围校验、通过 `SensorDataChan` 输出 |
| **wear_simulator** | `backend/internal/modules/wear_simulator/simulator.go` | Archard磨损计算、Dowson-Higginson EHL、混合润滑修正、生成油膜云图 |
| **life_predictor** | `backend/internal/modules/life_predictor/predictor.go` | Weibull分布贝叶斯估计、疲劳损伤累积、剩余寿命(RUL)预测 |
| **alarm_mqtt** | `backend/internal/modules/alarm_mqtt/pusher.go` | MQTT QoS=1 告警推送、30分钟冷却防抖、告警入库 |
| **scheduler** | `backend/internal/scheduler/scheduler.go` | 基于Channel的消息调度、结果监听、告警触发判断 |
| **monitoring** | `backend/internal/monitoring/metrics.go` | Prometheus指标(20+项)、net/http/pprof性能剖析 |
| **messages** | `backend/internal/modules/messages/messages.go` | 6种消息结构体、`ModuleChannels` 通道集合 |

### 前端模块

| 模块 | 文件 | 职责 |
|------|------|------|
| **Waterwheel3D** | `frontend/js/waterwheel_3d.js` | Three.js 筒车三维模型（轮毂、轮缘、24辐条、36水斗）、射线拾取交互 |
| **BearingPanel** | `frontend/js/bearing_panel.js` | 轴承剖面渲染、离屏Canvas动画纹理（三重差速旋转）、油膜颜色云图 |

### JSON 外置配置

| 文件 | 内容 |
|------|------|
| `backend/config/wear_params.json` | 22项磨损参数：Archard K基准、压粘系数、等效弹性模量、表面粗糙度、润滑系数因子、油膜网格配置 |
| `backend/config/lubrication_params.json` | 15项润滑参数：Sommerfeld数阈值、Greenwood-Williamson微凸体参数、Dowson-Higginson系数、油膜破裂阈值 |

---

## 快速部署

### 方式一：Docker Compose（推荐）

#### 1. 克隆项目并准备环境变量

```bash
git clone <repository-url>
cd AI_solo_coder_task_A_147
cp .env.example .env
# 根据需要修改 .env 中的密码
```

#### 2. 启动核心服务（Go后端 + TimescaleDB + MQTT Broker + Nginx前端）

```bash
docker-compose up -d --build
```

服务启动后访问：

| 服务 | URL |
|------|-----|
| 前端页面 | http://localhost/ |
| 后端API | http://localhost:8080/api/v1/health |
| API 健康检查 | http://localhost/api/health |
| MQTT Broker | mqtt://localhost:1883 |
| Modbus TCP | localhost:5020 |
| pprof | http://localhost:6060/debug/pprof/ |
| Prometheus metrics | http://localhost:9090/metrics |

#### 3. 启动传感器模拟器（可选）

```bash
# 正常工况
docker-compose run --rm sensor-simulator

# 极端工况 + 波动转速 + 快速模式
SIM_LOAD_PROFILE=extreme SIM_SPEED_PROFILE=surge SIM_INTERVAL=1 docker-compose run --rm sensor-simulator
```

#### 4. 启动监控栈（Prometheus + Grafana，可选）

```bash
docker-compose --profile monitoring up -d
```

访问 Grafana：http://localhost:3000 (admin / admin123)

#### 5. 停止服务

```bash
docker-compose down
# 保留数据卷
docker-compose down -v  # 删除数据卷也删除
```

### 方式二：本地开发

#### 前置依赖

- Go >= 1.21
- Python >= 3.9
- PostgreSQL >= 15 + TimescaleDB >= 2.13
- Eclipse Mosquitto >= 2.0

#### 步骤

```bash
# 1. 启动 TimescaleDB
# 执行 sql/init.sql 和 sql/timescale_policy.sql

# 2. 启动 MQTT Broker
mosquitto -c deploy/mosquitto/mosquitto.conf

# 3. 编译并启动后端
cd backend
go mod download
go run . config.yaml

# 4. 启动模拟器（另一个终端）
cd ../simulator
python noria_sensor_simulator.py --fast --load-profile normal

# 5. 启动前端
# 用任意HTTP服务器serve frontend目录
cd ../frontend
python -m http.server 8000
```

---

## 服务端口

| 端口 | 服务 | 说明 |
|------|------|------|
| 80 | Nginx | 前端静态资源 + API反向代理 + Gzip压缩 |
| 8080 | Go Gin API | 后端REST API |
| 5020 | Modbus TCP | 传感器数据上报 |
| 5432 | PostgreSQL | TimescaleDB数据库 |
| 1883 | MQTT | Mosquitto MQTT Broker |
| 9001 | MQTT WS | MQTT WebSocket |
| 6060 | pprof | Go性能剖析（内网）|
| 9090 | Prometheus | 指标采集 |
| 9091 | Prometheus UI | Prometheus Web界面（监控profile）|
| 3000 | Grafana | 监控仪表盘（监控profile）|

---

## 传感器模拟器

### 工况配置

#### 载荷工况 (`--load-profile`)

| 名称 | 基准载荷 | 磨损率 | 说明 |
|------|---------|--------|------|
| `light` | 2 000 N | 0.0010 μm/h | 枯水期低负荷 |
| `normal` | 5 000 N | 0.0030 μm/h | 常规水量（默认）|
| `heavy` | 10 000 N | 0.0080 μm/h | 汛期高负荷 |
| `extreme` | 18 000 N | 0.0200 μm/h | 洪水冲击+满载，加速磨损 |

#### 转速工况 (`--speed-profile`)

| 名称 | 基准转速 | 说明 |
|------|---------|------|
| `low` | 6 RPM | 枯水期慢转 |
| `normal` | 15 RPM | 常规水流（默认）|
| `high` | 30 RPM | 大流量 |
| `surge` | 20 RPM | 水流不稳定，转速大幅波动 |

### CLI 参数

```bash
python simulator/noria_sensor_simulator.py \
  --host 127.0.0.1 \
  --port 5020 \
  --bearing-id 1 \
  --load-profile normal \
  --speed-profile normal \
  --interval 3600 \
  --count 0
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--host` | 127.0.0.1 | Modbus TCP 目标地址 |
| `--port` | 5020 | Modbus TCP 端口 |
| `--bearing-id` | 1 | 轴承ID |
| `--load-profile` | normal | 载荷工况：light/normal/heavy/extreme |
| `--speed-profile` | normal | 转速工况：low/normal/high/surge |
| `--interval` | 3600 | 上报间隔秒数 |
| `--count` | 0 | 运行次数（0为无限）|
| `--once` | - | 只发送一次 |
| `--fast` | - | 快速模式：1秒≈1小时加速仿真 |
| `--list-profiles` | - | 列出所有工况 |
| `--json` | - | JSON格式输出 |

### 使用示例

```bash
# 列出所有可用工况
python simulator/noria_sensor_simulator.py --list-profiles

# 快速仿真1次数据（测试用）
python simulator/noria_sensor_simulator.py --once --fast --json

# 重载波动转速，1秒1条（≈1小时）连续运行
python simulator/noria_sensor_simulator.py --fast --load-profile heavy --speed-profile surge

# Docker方式运行极端工况
docker-compose run --rm -e SIM_LOAD_PROFILE=extreme -e SIM_SPEED_PROFILE=surge sensor-simulator
```

---

## 监控与运维

### Prometheus 指标

Go服务暴露了 20+ 项 Prometheus 指标，访问 `http://localhost:9090/metrics
：

| 指标类型 | 指标名称 | 说明 |
|---------|----------|------|
| Counter | `noria_modbus_received_total` | Modbus接收数据包数 |
| Counter | `noria_modbus_validation_fail_total` | 数据校验失败数 |
| Counter | `noria_wear_calculations_total` | 磨损计算次数 |
| Counter | `noria_wear_calc_errors_total` | 磨损计算错误数 |
| Gauge | `noria_wear_depth_microm` | 当前累计磨损深度(μm) |
| Gauge | `noria_ehl_film_parameter_lambda` | EHL膜厚参数 Lambda |
| Counter | `noria_life_predictions_total` | 寿命预测次数 |
| Gauge | `noria_predicted_rul_hours` | 预测剩余寿命（小时）|
| Gauge | `noria_weibull_shape_parameter` | Weibull形状参数 Beta |
| Gauge | `noria_weibull_scale_parameter` | Weibull尺度参数 Eta |
| Gauge | `noria_bearing_reliability` | 轴承可靠度(0-1) |
| Counter | `noria_alerts_sent_total` | 已发送告警数 |
| Counter | `noria_alerts_suppressed_total` | 冷却机制抑制的告警数 |
| Gauge | `noria_bearing_temperature_celsius` | 轴承温度(°C) |
| Gauge | `noria_bearing_radial_load_newtons` | 径向载荷(N) |
| Gauge | `noria_bearing_rotational_speed_rpm` | 转速(RPM) |
| Gauge | `noria_bearing_oil_film_thickness_microm` | 油膜厚度(μm) |
| CounterVec | `noria_http_requests_total` | HTTP请求数（带method/path/status标签）|
| Histogram | `noria_http_request_duration_seconds` | HTTP请求耗时分布 |
| GaugeVec | `noria_channel_depth` | 各通信通道深度 |

### pprof 性能剖析

Go的`http://localhost:6060/debug/pprof/
提供标准 net/http/pprof）：

```bash
# 30秒CPU Profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 堆内存
go tool pprof http://localhost:6060/debug/pprof/heap

# goroutine
go tool pprof http://localhost:6060/debug/pprof/goroutine

# 火焰图
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/profile?seconds=30
```

### TimescaleDB 数据保留策略

| 数据类型 | 保留时长 | 说明 |
|---------|---------|------|
| 原始 sensor_data | 90 天 | 高频原始采样数据 |
| 5 分钟聚合 | 30 天 | 实时监控用 |
| 15 分钟聚合 | 90 天 | 短期趋势 |
| 1 小时聚合 | 2 年 | 中期分析 |
| 1 天聚合 | 10 年 | 长期历史趋势 |
| wear_results | 5 年 | 磨损计算原始结果 |
| 磨损日聚合 | 10 年 | 磨损长期统计 |
| life_predictions | 5 年 | 寿命预测原始结果 |
| alert_events | 3 年 | 告警历史 |
| 告警日统计 | 10 年 | 告警统计 |
| oil_film_maps | 90 天 | 油膜云图（JSONB较大）|

查看当前状态检查SQL函数：

```sql
-- 查看所有保留策略
SELECT * FROM show_retention_policies();

-- 查看所有连续聚合策略
SELECT * FROM show_cagg_policies();
```

### Nginx Gzip 压缩

Nginx 已启用 Gzip 压缩（见 `deploy/nginx/gzip.conf`

压缩级别 6，压缩类型：
- text/plain, text/css, text/javascript, application/javascript
- application/json, application/xml, image/svg+xml
- 字体文件（woff/ttf/eot）等

---

## API 接口

### 健康检查

```
GET /api/health
GET /health
```

响应示例：

```json
{
  "status": "ok",
  "timestamp": "2024-06-15T10:30:00Z",
  "service": "noria-bearing-system",
  "version": "v1.0.0",
  "database": { "connected": true }
}
```

### 核心API

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/v1/noria-wheels` | 筒车列表 |
| GET | `/api/v1/bearings` | 轴承列表 |
| GET | `/api/v1/bearings/:id` | 轴承详情 |
| GET | `/api/v1/bearings/status` | 所有轴承最新状态 |
| GET | `/api/v1/bearings/:bearing_id/sensor-data` | 传感器历史数据 |
| GET | `/api/v1/bearings/:bearing_id/sensor-data/latest` | 最新传感器数据 |
| POST | `/api/v1/sensor-data` | 手动上报传感器数据 |
| GET | `/api/v1/bearings/:bearing_id/wear-history` | 磨损历史 |
| GET | `/api/v1/bearings/:bearing_id/wear/latest` | 最新磨损结果 |
| GET | `/api/v1/bearings/:bearing_id/life-prediction/latest` | 最新寿命预测 |
| GET | `/api/v1/bearings/:bearing_id/oil-film-map` | 油膜云图数据 |
| POST | `/api/v1/calculations/trigger` | 触发一次磨损+寿命计算 |
| GET | `/api/v1/alerts/recent` | 最近告警事件 |
| GET | `/api/v1/debug/weibull` | Weibull拟合调试接口 |

### MQTT 告警主题

```
Topic: noria/alerts/<bearing_id>/<alert_type>
```

消息格式：

```json
{
  "bearing_id": 1,
  "bearing_code": "NRW-001-BR-A",
  "alert_type": "wear_exceeded",
  "alert_level": "critical",
  "message": "磨损深度超过阈值",
  "threshold": 150.0,
  "actual_value": 156.3,
  "timestamp": "2024-06-15T10:30:00Z"
}
```

---

## 目录结构

```
AI_solo_coder_task_A_147/
├── backend/
│   ├── main.go                          # 程序入口
│   ├── go.mod / go.sum
│   ├── config.yaml                      # 主配置文件
│   ├── config/
│   │   ├── wear_params.json            # 磨损参数（外置）
│   │   └── lubrication_params.json     # 润滑参数（外置）
│   └── internal/
│   │   ├── api/handlers.go           # Gin API处理器
│   │   ├── config/config.go         # 配置加载
│   │   ├── database/db.go           # 数据库
│   │   ├── models/models.go         # 数据模型
│   │   ├── monitoring/metrics.go    # Prometheus + pprof
│   │   ├── scheduler/scheduler.go     # Channel调度器
│   │   └── modules/
│   │       ├── messages/messages.go        # 消息结构体
│   │       ├── modbus_receiver/       # Modbus采集校验
│   │       ├── wear_simulator/       # Archard+EHL磨损计算
│   │       ├── life_predictor/      # Weibull+RUL寿命预测
│   │       └── alarm_mqtt/      # MQTT告警推送
├── frontend/
│   ├── index.html
│   ├── css/style.css
│   └── js/
│       ├── waterwheel_3d.js          # 筒车三维渲染
│       ├── bearing_panel.js        # 轴承剖面+云图
│       ├── app.js / api.js / charts.js / colormap.js
│       └── oilfilm-view.js
├── simulator/
│   └── noria_sensor_simulator.py  # Modbus传感器模拟器
├── sql/
│   ├── init.sql                   # 数据库初始化（表+超表）
│   └── timescale_policy.sql         # 降采样+保留策略
├── deploy/
│   ├── nginx/
│   │   ├── default.conf            # Nginx站点配置+反代+Gzip
│   │   └── gzip.conf               # Gzip压缩配置
│   ├── mosquitto/
│   │   └── mosquitto.conf           # MQTT Broker配置
│   ├── prometheus/
│   │   └── prometheus.yml       # Prometheus采集配置
│   └── grafana/
│       ├── datasource.yml            # 数据源配置
│       └── dashboards/              # 仪表盘JSON
├── Dockerfile                      # Go多阶段构建
├── docker-compose.yml
├── .env.example
└── README.md
└── regression_test.py / verify_code.py
```

---

## License

Internal use only - 水利史研究项目
