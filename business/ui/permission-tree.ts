import type { CheckboxTreeNode } from '@liveshop/design-tokens'

export interface PermissionTreeItem {
  moduleId?: string
  code: string
  name: string
  resource: string
  action: string
}

export function permissionTree(permissions: PermissionTreeItem[]): CheckboxTreeNode[] {
  const modules = new Map<string, Map<string, PermissionTreeItem[]>>()
  for (const permission of permissions) {
    const moduleId = permission.moduleId || permission.code.split('.')[0] || '其他模块'
    const resources = modules.get(moduleId) ?? new Map<string, PermissionTreeItem[]>()
    const resource = permission.resource || '其他资源'
    resources.set(resource, [...(resources.get(resource) ?? []), permission])
    modules.set(moduleId, resources)
  }
  return [...modules.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([moduleId, resources]) => {
      const resourceNodes = [...resources.entries()]
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([resource, items]) => ({
          id: `resource:${moduleId}:${resource}`,
          label: resource,
          description: `${items.length} 项权限`,
          children: items
            .slice()
            .sort((left, right) => left.code.localeCompare(right.code))
            .map(permission => ({
              id: `permission:${permission.code}`,
              label: permission.name || permission.action || permission.code,
              value: permission.code,
              description: permission.code,
            })),
        }))
      const count = resourceNodes.reduce((total, resource) => total + (resource.children?.length ?? 0), 0)
      return {
        id: `module:${moduleId}`,
        label: moduleId,
        description: `${count} 项权限`,
        children: resourceNodes,
      }
    })
}
