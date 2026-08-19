import type { RemoteModule, RemoteModuleContext } from '@liveshop/host-sdk'
import { renderPlaceholder } from '../../ui/placeholder'
import { renderAddressBook, renderAddressEdit, renderFavorites } from './pages/customer'
import { renderAftersaleDetail, renderAftersales } from './pages/aftersales'
import { renderIdentityScaffold } from './scaffold'
import styles from './style.css?inline'

const contribution: RemoteModule = {
  mount(container: HTMLElement, context: RemoteModuleContext): void {
    const style = document.createElement('style')
    style.textContent = styles
    const root = document.createElement('div')
    container.replaceChildren(style, root)
    if (context.contributionId === 'identity.shop.address') {
      void renderAddressBook(root, context)
      return
    }
    if (context.contributionId === 'identity.shop.address-edit') {
      void renderAddressEdit(root, context)
      return
    }
    if (context.contributionId === 'identity.shop.favorites') {
      void renderFavorites(root, context)
      return
    }
    if (context.contributionId === 'identity.shop.aftersales') {
      void renderAftersales(root, context)
      return
    }
    if (context.contributionId === 'identity.shop.aftersale-detail') {
      void renderAftersaleDetail(root, context)
      return
    }
    if (context.contributionId.includes('.shop.')) renderIdentityScaffold(root, context)
    else renderPlaceholder(root, context)
  },
  unmount(container: HTMLElement): void { container.replaceChildren() },
}

export default contribution
