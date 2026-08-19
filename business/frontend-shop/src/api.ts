import type { HostHttpClient } from '@liveshop/host-sdk'

const ROOT = '/shop/identity'

export interface Address {
  id: number
  recipient: string
  phone: string
  country: string
  province: string
  city: string
  district: string
  detail: string
  postalCode: string
  isDefault: boolean
  version: number
}

export interface AddressInput {
  recipient: string
  phone: string
  country: string
  province: string
  city: string
  district: string
  detail: string
  postalCode: string
  isDefault: boolean
}

export interface WishlistItem {
  productId: number
  createdAt: number
}

export interface Profile {
  subject: string
  principalType: string
  signedIn: boolean
  displayName: string
}

export interface SMSRegion {
  dialCode: string
  name: string
  iso2: string
  emoji: string
}

export interface AftersaleItem {
  id: number
  skuId: number
  title: string
  quantity: number
  refundAmount: number
  receivedQuantity: number
}

export interface Aftersale {
  id: number
  orderId: number
  paymentNo: string
  type: string
  requestedAmount: number
  amount: number
  reason: string
  status: string
  returnStatus: string
  handleNote: string
  items: AftersaleItem[]
  version: number
  createdAt: string
  updatedAt: string
  reviewedAt?: string
  receivedAt?: string
}

export class IdentityShopApi {
  constructor(private readonly client: HostHttpClient) {}

  loginSMSRegions(shopCode: string): Promise<{ items: SMSRegion[]; unrestricted: boolean }> {
    return this.client.request(`${ROOT}/login/sms-regions?shopCode=${encodeURIComponent(shopCode)}`)
  }

  profile(): Promise<Profile> {
    return this.client.request(`${ROOT}/profile`)
  }

  aftersales(status = ''): Promise<{ items: Aftersale[]; total: number }> {
    const query = status ? `?status=${encodeURIComponent(status)}` : ''
    return this.client.request(`${ROOT}/aftersales${query}`)
  }

  aftersale(id: number): Promise<{ aftersale: Aftersale }> {
    return this.client.request(`${ROOT}/aftersales/${id}`)
  }

  addresses(): Promise<{ items: Address[] }> {
    return this.client.request(`${ROOT}/addresses`)
  }

  createAddress(input: AddressInput, commandKey: string): Promise<Address> {
    return this.client.request(`${ROOT}/addresses`, {
      method: 'POST',
      body: JSON.stringify({ ...input, commandKey }),
    })
  }

  updateAddress(id: number, input: AddressInput, commandKey: string, expectedVersion: number): Promise<Address> {
    return this.client.request(`${ROOT}/addresses/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ ...input, commandKey, expectedVersion }),
    })
  }

  deleteAddress(id: number, commandKey: string, expectedVersion: number): Promise<{ ok: boolean }> {
    return this.client.request(`${ROOT}/addresses/${id}`, {
      method: 'DELETE',
      body: JSON.stringify({ commandKey, expectedVersion }),
    })
  }

  replaceDefault(addressId: number, commandKey: string, expectedVersion: number): Promise<Address> {
    return this.client.request(`${ROOT}/addresses/default`, {
      method: 'PUT',
      body: JSON.stringify({ addressId, commandKey, expectedVersion }),
    })
  }

  wishlist(): Promise<{ items: WishlistItem[] }> {
    return this.client.request(`${ROOT}/wishlist?limit=100`)
  }

  addWishlist(productId: number, commandKey: string): Promise<WishlistItem> {
    return this.client.request(`${ROOT}/wishlist/items`, {
      method: 'POST',
      body: JSON.stringify({ productId, commandKey }),
    })
  }

  removeWishlist(productId: number): Promise<{ ok: boolean }> {
    return this.client.request(`${ROOT}/wishlist/items/${productId}`, { method: 'DELETE' })
  }
}
