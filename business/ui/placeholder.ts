import { badge, dataCard, definitionList, page } from '@liveshop/design-tokens'

export interface PlaceholderContext {
  moduleId: string
  moduleVersion: string
  contributionId: string
  surface: string
}

export function renderPlaceholder(container: HTMLElement, context: PlaceholderContext): void {
  container.replaceChildren(page({
    showSummary: false,
    children: [dataCard({
      title: '迁移状态',
      body: definitionList([
        { label: '页面状态', value: badge({ label: '待迁移', tone: 'warning' }) },
        { label: '所属模块', value: context.moduleId },
        { label: '交付端', value: context.surface },
        { label: '模块版本', value: context.moduleVersion },
        { label: 'Contribution', value: context.contributionId },
        { label: '后端权限', value: '未授予业务 API 路由范围' },
        { label: '当前限制', value: '领域契约、真实接口和数据源完成前，不提供业务操作或模拟数据。' },
      ]),
    })],
  }))
}

