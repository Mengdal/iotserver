# 看板组件（viz widgets）

画布看板 = **布局 + 组件**。布局由 `components/GridCanvas`（gridstack 封装）负责，
组件本身由这里的**注册表**驱动：每个组件类型自己声明"叫什么、怎么渲染、怎么配置、怎么取数"。

```
src/components/viz/
├── types.ts           组件契约（VizWidgetManifest / VizWidgetContext）
├── registry.ts        注册表：登记所有组件类型 + 查询接口
├── WidgetBody.tsx     按类型渲染组件内容（合并默认配置后交给 manifest.render）
├── useVizData.ts      批量取数 hook（画布一次请求取回所有数据类组件的数据）
├── format.ts          数值/时间/序列名格式化
└── widgets/
    ├── TextWidget.tsx        文本
    ├── misc.tsx              容器（layoutOnly）、早期图表占位
    ├── StatsWidget.tsx       统计卡（一卡多个指标块）
    ├── KpiWidget.tsx         数值卡
    ├── GaugeWidget.tsx       仪表盘（阈值配色）
    ├── LineWidget.tsx        折线图（@ant-design/plots）
    ├── TableWidget.tsx       表格（当前值 / 历史明细）
    └── SeriesEditor.tsx      共用的"设备 + 属性"选择器
```

## 新增一种组件只需要两步

1. 在 `widgets/` 下新建文件，导出一个 manifest：

```tsx
export const myManifest: VizWidgetManifest<MyConfig> = {
  type: 'mywidget',             // 与后端 vizWidgetRegistry 的 key 一致
  label: '我的组件',
  icon: <StarOutlined />,
  defaultSize: { w: 4, h: 6 },
  data: true,                   // 数据类组件才需要下面两项
  defaultConfig: () => ({ series: [], ... }),
  buildQuery: config => ({ series: config.series, latest: true, ... }),
  render: ({ config, ctx }) => <MyWidget config={config} ctx={ctx} />,
  Settings: MySettings,         // 组件自己的配置面板（不写就只有标题等通用项）
}
```

2. 在 `registry.ts` 的 `manifests` 数组里加一行。

组件库列表、默认标题、默认尺寸、配置抽屉都会自动出现，**不需要改看板编辑器**。

## 数据是怎么到组件里的

- 看板页/只读大屏：`useVizData(组件列表, 刷新间隔)` 汇总所有数据类组件的 `buildQuery()`
  结果，调一次 `POST /api/v1/viz/device-query`，后端并发查完返回 `{results: [{id, series[]}]}`。
- 结果按组件 id 挂在 `ctx.data` 上，组件只管渲染，不关心请求。
- 刷新按**组件**调度：每个组件生效的间隔 = 组件自己的 `widget.refreshInterval` → 否则看板的
  `canvas.refreshInterval`（都为 0 就不自动刷新）。hook 每秒挑出到期的组件，把同一轮
  到期的合并成一次请求，静默重取（不显示 loading，避免闪屏）。
  所以一块看板上可以有"10 秒的当前值卡片"和"1 小时的趋势曲线"并存。
- 编辑态不自动轮询，手动点「刷新数据」；配置面板「基本设置」里可以单独调这个组件的间隔。

## 配置的存法

- 组件的自由配置存在 `widget.config`（后端是 `map[string]interface{}`，只做默认值合并与大小校验）。
- 文本正文是组件的独立字段 `widget.text`（不是 config），通过 `ctx.text / ctx.onTextChange` 读写。
- 后端 `vizWidgetRegistry` 会补默认值、校验数据序列是否成对（deviceId + code），
  并**原样保留未知类型**：新版本前端建的组件在旧版本后端上不会被改写或丢数据。

## 首页 / 内置看板 / 视图可见性

- **首页（/home/dashboard）整页渲染「首页看板」**（`slot=home` 的那块，见 `pages/home/Dashboard`），
  不再维护手写 UI；「看板管理」里可随时换一块。
- 某个租户**还没有任何看板**时，后端自动种入内置看板（首页 / 组件演示 / 系统监控，
  编译期内嵌 `internal/pkg/seed/boards`）：平台 3 块、普通租户 2 块（演示看板仅平台）。
- 指标带 `platformOnly`（系统资源 `system.*`、租户总数等）：租户视图下不下发，
  绑它的**统计块/整个组件自动隐藏**（判定见 `components/viz/platformScope.ts`），
  同一块「系统监控看板」因此能做到"平台管理员看系统资源，租户看设备运行"。

## 后端配套

| 接口 | 作用 |
| --- | --- |
| `GET /api/v1/viz/boards`、`GET/PUT/DELETE /api/v1/viz/boards/:id` | 看板增删改查（画布 JSON 落 `viz_board` 表，按租户隔离） |
| `POST /api/v1/viz/device-query` | 批量取数（设备属性 + 平台指标，支持时间范围与聚合） |
| `GET /api/v1/viz/metrics` | 可绑定指标清单（含 `kind=series` 的时序指标与 `platformOnly` 标记） |
| `POST /api/v1/viz/boards/:id/active` | 设为首页看板 / 数据看板整页 |

取数实现见 `iotserver/internal/iotserver/core/controller/http/gateway/viz_data.go`，
走平台自己的 DuckDB 数据层，
支持 `aggInterval` + `aggMethod` 库内分桶。

> ⚠️ 一个容易踩的坑：数据层在「不传时间范围 + `Last`」时只回溯最近 30 分钟
> （`duckdb/helpers.go` 的 `timeWindow`），
> 设备半小时没上报就会查空。所以取数接口在 latest 模式下**总是显式给回看窗口**。

## 权限

| 权限 | 用途 |
| --- | --- |
| `viz:board:list` / `viz:board:query` | 看板列表 / 详情（数据看板页也要） |
| `viz:data:query` | 看板取数（没有它只能看排版，看不到设备数据） |
| `viz:board:add` / `viz:board:edit` / `viz:board:remove` | 看板管理页 |

权限的唯一来源是 `iotserver/internal/pkg/permissions/permissions.go`，
前端常量由 `go run ./tools/gen-permissions` 生成到 `src/routes/permissions.ts`。
