import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { button, dataCard, page, statusLine, table } from '@liveshop/design-tokens'
import { permissionTree } from '../../../ui/permission-tree'

interface Role { id:number; code:string; name:string; status:string; systemRole:boolean; version:number }
interface Permission { code:string; name:string; resource:string; action:string; description:string }

const permissionCodes = (value:string):string[] => [...new Set(value.split(/[\s,]+/).map(code => code.trim()).filter(Boolean))].sort()

export async function renderAuthorization(root:HTMLElement, api:HostHttpClient, prefix='/admin/identity'):Promise<void> {
  const state=statusLine()
  const rolesTable=table({columns:['ID','稳定角色码','名称','状态','系统保护','版本']})
  const permissionsTable=table({columns:['权限码','名称','资源','动作']})
  let roles:Role[]=[]
  let permissions:Permission[]=[]
  const role=hostFormModal({title:'角色',fields:[{name:'id',label:'角色 ID',type:'number',required:true},{name:'expectedVersion',label:'期望版本（新建 0）',type:'number',required:true,value:0},{name:'code',label:'稳定角色码 / 旧 role_key',required:true},{name:'name',label:'名称',required:true},{name:'status',label:'状态',kind:'select',options:['ACTIVE','DISABLED'],required:true}],onSubmit:(v,m)=>{m.setBusy(true);api.request(`${prefix}/authorization/roles/${Number(v.id)}`,{method:'PUT',body:JSON.stringify({expectedVersion:Number(v.expectedVersion),code:v.code,name:v.name,status:v.status})}).then(()=>{m.close();return load()}).catch(e=>m.setError(String(e))).finally(()=>m.setBusy(false))}})

  function openPolicy():void {
    if(!permissions.length){state.set('活动 Registry 中没有可分配权限；工作负载专用权限不会出现在这里。','warning');return}
    const modal=hostFormModal({title:'角色策略',fields:[
      {name:'id',label:'角色',kind:'select',options:roles.map(r=>({value:r.id,label:`${r.name} · ${r.code}`})),required:true},
      {name:'expectedVersion',label:'期望版本',type:'number',required:true},
      {name:'permissionCodes',label:'授权权限',kind:'checkbox-tree',tree:permissionTree(permissions),wide:true,empty:'当前没有可分配的活动权限'},
      {name:'resource',label:'资源码',required:true},
      {name:'scope',label:'数据范围',kind:'select',options:['ALL','SELF','CURRENT_ORG_UNIT','ORG_UNIT_SUBTREE','CURRENT_SHOP','ASSIGNED_SHOPS'],required:true},
    ],onSubmit:(v,m)=>{
      const selected=permissionCodes(v.permissionCodes)
      const catalog=new Set(permissions.map(p=>p.code))
      const unknown=selected.filter(code=>!catalog.has(code))
      if(!selected.length){m.setError('至少选择一个权限码');return}
      if(unknown.length){m.setError(`权限码不在活动 Registry：${unknown.join(', ')}`);return}
      m.setBusy(true)
      api.request(`${prefix}/authorization/roles/${Number(v.id)}/policy`,{method:'PUT',body:JSON.stringify({expectedVersion:Number(v.expectedVersion),permissions:selected,scopes:[{resource:v.resource,type:v.scope,referenceIds:[]}]})}).then(()=>{m.close();return load()}).catch(e=>m.setError(String(e))).finally(()=>m.setBusy(false))
    }})
    modal.open()
  }

  function openGrant():void {
    if(!roles.length){state.set('没有可授权的角色。','warning');return}
    const modal=hostFormModal({title:'主体授权',fields:[{name:'subject',label:'Identity Subject',required:true},{name:'roleId',label:'角色',kind:'select',options:roles.map(r=>({value:r.id,label:`${r.name} · ${r.code}`})),required:true},{name:'accessVersion',label:'成员 accessVersion',type:'number',required:true}],onSubmit:(v,m)=>{m.setBusy(true);api.request(`${prefix}/authorization/subjects/${encodeURIComponent(v.subject)}/grants`,{method:'PUT',body:JSON.stringify({roleIds:[Number(v.roleId)],operationId:crypto.randomUUID(),accessVersion:Number(v.accessVersion)})}).then(()=>{m.close();state.set('授权已保存','success')}).catch(e=>m.setError(String(e))).finally(()=>m.setBusy(false))}})
    modal.open()
  }

  async function load(){try{[roles,permissions]=await Promise.all([api.request<Role[]>(`${prefix}/authorization/roles`),api.request<Permission[]>(`${prefix}/authorization/permissions`)]);rolesTable.setRows(roles.map(x=>[x.id,x.code,x.name,x.status,x.systemRole?'是':'否',x.version]));permissionsTable.setRows(permissions.map(x=>[x.code,x.name,x.resource,x.action]));state.set(`角色 ${roles.length} 个 · 可分配权限 ${permissions.length} 个`)}catch(e){state.set(`加载失败：${String(e)}`,'danger')}}
  root.replaceChildren(page({showSummary:false,children:[dataCard({title:'角色列表',actions:[button({label:'刷新',variant:'secondary',onClick:()=>void load()}),button({label:'新建角色',onClick:()=>role.open()}),button({label:'编辑策略',variant:'secondary',onClick:openPolicy}),button({label:'主体授权',variant:'secondary',onClick:openGrant})],status:state.element,body:rolesTable.element}),dataCard({title:'活动可分配权限',body:permissionsTable.element})]}))
  await load()
}
