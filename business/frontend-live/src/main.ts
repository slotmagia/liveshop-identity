import type { RemoteModule, RemoteModuleContext } from '@liveshops/host-sdk'
import { renderPlaceholder } from '../../ui/placeholder'
import styles from './style.css?inline'

const contribution: RemoteModule = {
  mount(container: HTMLElement, context: RemoteModuleContext): void {
    const style = document.createElement('style')
    style.textContent = styles
    const root = document.createElement('div')
    container.replaceChildren(style, root)
    renderPlaceholder(root, context)
  },
  unmount(container: HTMLElement): void { container.replaceChildren() },
}

export default contribution
