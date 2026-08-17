# Identity merch 贡献规则

本目录是 `merch` surface 的前端贡献，产物类型是 **iframe**，由 `merch` Host 加载。

## 不可违反

- **凭证只来自 Host。** iframe 走 `connectToHost()` 握手，remote-esm 走 `mount(container, context)` 的入参。禁止从 `localStorage`、cookie 或 URL 读 token——那些渠道无法证明是哪个 Host 在请求。
- **不要自己拼 Gateway 地址或加 Authorization 头。** 用 `@liveshop/host-sdk` 的客户端，它负责注入 token 和 `X-Liveshop-Surface`。
- **请求路径必须在 `module.json` 的 `allowedRoutes` 里。** session 的路由范围由 contribution 决定，漏登记的路径线上会 403，本地却可能因为权限宽松而看不出来。
- **产物类型不能改。** `merch` Host 只会以 iframe 方式加载；改了 `vite.config.ts` 的构建形态就加载不了。
- **界面只能来自共享组件库。** 用 `@liveshop/design-tokens` 的组件工厂构建 DOM（后台是包根导出，商城和直播是 `/storefront` 子路径）；本目录只写本领域特有的布局，禁止重新定义按钮、表单、卡片、表格、价格、状态或弹窗的视觉——两个后台当初就是这样各自漂移开的。
- **不要写死颜色和字号**，用 `@liveshop/design-tokens` 的变量；也不要写 `var(--ls-x, #fallback)`，令牌缺失必须直接暴露，而不是悄悄退回第二套配色。
- **不要为静态结构拼 `innerHTML`。** 组件工厂一律用 `textContent` 写值，服务端字符串因此不可能进入 HTML 解析器。
- **不要跨 surface 复用页面模块。** 每个 surface 的权限、布局和 Host 能力都不同，共用会让权限边界变模糊。

## 与后端的对应

页面调用的每个接口，都要在 `../module.json` 里同时有：`backend.httpRoutes` 的 operation、contribution 的 `allowedRoutes` 条目、以及匹配的权限码。三者缺一就是线上 403 或 404。

## 本地开发

```powershell
npm install
npm run dev     # 127.0.0.1:5191
npm run build   # tsc --noEmit + vite build
```

`@liveshop/host-sdk` 与 `@liveshop/design-tokens` 通过相对 `file:` 依赖引用 `liveshop-platform`，因此**本仓库必须与 `liveshop-platform` 同级**。发布构建需要这两个包已发布到可访问的 registry。

页面本身不能脱离 Host 独立跑通：没有握手就没有 session。要看真实效果，把 `merch` Host 指向上面的 dev 端口。

## 模态框与遮罩

- 后台 iframe 的简单表单只能使用 `hostFormModal()`；富内容对话框只能使用共享 `modal()`，并在打开/关闭时成对调用 `hostOverlay()`。禁止自建 modal/backdrop 组件。
- 遮罩必须由 Host 覆盖整个应用视口（`100vw × 100dvh`）。禁止用 HTML5 Fullscreen、`window.top` 或向父页面注入 DOM 绕过 Host 协议。
- 模态框结构固定为 Header / Body / Footer：Header 和 Footer 永不参与滚动，只有 Body 可以 `overflow-y: auto`；页面和对话框外壳不得出现第二条滚动条。
- 套餐权益、员工角色、店铺应用授权范围等集合必须使用 Host 表单的 `kind: 'checkbox-tree'`，树叶提交稳定权限码，父节点支持整组选择和半选；禁止 textarea 手工权限码。

## 菜单说明卡片

- 页面标题和描述只在活动 Manifest contribution 中维护，由 Host 在业务内容上方统一渲染卡片。
- 本 contribution 使用共享 `page()` 时必须设置 `showSummary: false`，不得复制页面级标题和描述。
- 管理型页面有查询条件时必须使用独立 `searchCard()`；表格、树或集合必须使用 `dataCard()`。
- 查询字段变化后由共享 `searchForm()` 自动搜索，不得为查询字段自写 `change`/`input` 监听。级联下拉必须在 `onSearch` 内同步选项，替换 option 后调用 `refreshSearchSelect()`。`kind: 'select'` 使用共享可搜索下拉，点选即选中；空值表示全部。菜单 portal 到当前文档 `body`，`--ls-z-popover` 必须高于 Host 表单模态遮罩。
- 成功、失败等瞬时反馈只用 `notify()`。新页面不要把 `statusLine().element` 放进 `dataCard`。
- 刷新、新增、导入、导出和批量操作只能放在 `dataCard()` 的表格工具栏；单行编辑/启停操作留在行内。禁止 `ls-ui-page-toolbar` 悬空工具栏。
