import { connectToHost, createHttpClient } from '@liveshop/host-sdk'

import { renderAccount } from './pages/account'
import { renderSecurity } from './pages/security'
import { renderDevices } from './pages/devices'
import { renderProfile } from './pages/profile'
import { renderUsers } from './pages/users'
import { renderAuthorization } from './pages/authorization'
import { renderPrivacy } from './pages/privacy'
import { renderPolicies } from './pages/policies'
import { renderApps } from './pages/apps'
import { renderShops } from './pages/shops'
import { renderShopSettings } from './pages/shop-settings'
import { renderPlans } from './pages/plans'
import { renderBilling } from './pages/billing'
import { renderRiskEvents } from './pages/risk-events'
import { renderPlaceholder } from '../../ui/placeholder'
import './style.css'

// The Host embeds this app in an iframe and hands over the session through a
// postMessage handshake. Never read a token from storage or the URL: the
// handshake is the only channel that proves which Host is asking.
const root = document.querySelector<HTMLElement>('#app')

if (!root) throw new Error('missing #app container')

void connectToHost()
  .then(context => {
    if (context.contributionId === 'identity.merch.authorization') return renderAuthorization(root, createHttpClient(context), '/merch/identity')
    if (context.contributionId === 'identity.merch.organization') return renderAccount(root, createHttpClient(context))
    if (context.contributionId === 'identity.merch.account-security') return renderSecurity(root, createHttpClient(context))
    if (context.contributionId === 'identity.merch.devices') return renderDevices(root, createHttpClient(context))
    if (context.contributionId === 'identity.merch.profile') return renderProfile(root, createHttpClient(context))
    if (context.contributionId === 'identity.merch.users') return renderUsers(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.privacy') return renderPrivacy(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.policies') return renderPolicies(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.apps') return renderApps(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.shops') return renderShops(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.settings-general') return renderShopSettings(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.plans') return renderPlans(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.billing') return renderBilling(root, createHttpClient(context), context)
    if (context.contributionId === 'identity.merch.risk-events') return renderRiskEvents(root, createHttpClient(context))
    return renderPlaceholder(root, context)
  })
  .catch(error => {
    root.textContent = error instanceof Error ? error.message : String(error)
  })
