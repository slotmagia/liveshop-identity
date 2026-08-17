import type { RemoteModule, RemoteModuleContext } from '@liveshop/host-sdk'
import { renderPlaceholder } from '../../ui/placeholder'
import { renderIdentityScaffold } from './scaffold'
import styles from './style.css?inline'

const contribution: RemoteModule = {
  mount(container: HTMLElement, context: RemoteModuleContext): void {
    const style = document.createElement('style')
    style.textContent = styles
    const root = document.createElement('div')
    container.replaceChildren(style, root)
    if (context.contributionId.includes('.shop.')) renderIdentityScaffold(root, context)
    else renderPlaceholder(root, context)
  },
  unmount(container: HTMLElement): void { container.replaceChildren() },
}

export default contribution
