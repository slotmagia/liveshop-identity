import { connectToHost, createHttpClient } from '@liveshop/host-sdk'

import { renderDirectory } from './pages/directory'
import { renderMerchants } from './pages/merchants'
import { renderAuthorization } from './pages/authorization'
import { renderSubscription } from './pages/subscription'
import { renderShops } from './pages/shops'
import { renderShopCategories } from './pages/shop-category'
import { renderCustomerAccounts } from './pages/customer-accounts'
import { renderSettingsGovernance } from './pages/settings-governance'
import { renderPlaceholder } from '../../ui/placeholder'
import './style.css'

// The Host embeds this app in an iframe and hands over the session through a
// postMessage handshake. Never read a token from storage or the URL: the
// handshake is the only channel that proves which Host is asking.
const root = document.querySelector<HTMLElement>('#app')

if (!root) throw new Error('missing #app container')

void connectToHost()
  .then(context => {
    if (context.contributionId === 'identity.admin.authorization') return renderAuthorization(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.users') return renderDirectory(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.admin.merchants') return renderMerchants(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.subscription') return renderSubscription(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.shops') return renderShops(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.shop-categories') return renderShopCategories(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.customer-accounts') return renderCustomerAccounts(root, createHttpClient(context))
    if (context.contributionId === 'identity.admin.settings-governance') return renderSettingsGovernance(root, createHttpClient(context))
    return renderPlaceholder(root, context)
  })
  .catch(error => {
    root.textContent = error instanceof Error ? error.message : String(error)
  })
