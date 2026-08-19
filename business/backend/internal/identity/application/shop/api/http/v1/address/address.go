package address

import "github.com/gogf/gf/v2/frame/g"

type Item struct {
	ID         int64  `json:"id"`
	Recipient  string `json:"recipient"`
	Phone      string `json:"phone"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	District   string `json:"district"`
	Detail     string `json:"detail"`
	PostalCode string `json:"postalCode"`
	IsDefault  bool   `json:"isDefault"`
	Version    uint64 `json:"version"`
}

type ListReq struct {
	g.Meta `path:"/addresses" method:"get" tags:"Identity-shop" summary:"List customer addresses"`
}

type ListRes struct {
	Items []Item `json:"items"`
}

type CreateReq struct {
	g.Meta     `path:"/addresses" method:"post" tags:"Identity-shop" summary:"Create a customer address"`
	Recipient  string `json:"recipient"`
	Phone      string `json:"phone"`
	Country    string `json:"country"`
	Province   string `json:"province"`
	City       string `json:"city"`
	District   string `json:"district"`
	Detail     string `json:"detail"`
	PostalCode string `json:"postalCode"`
	IsDefault  bool   `json:"isDefault"`
	CommandKey string `json:"commandKey"`
}

type CreateRes Item

type UpdateReq struct {
	g.Meta          `path:"/addresses/{addressId}" method:"put" tags:"Identity-shop" summary:"Update a customer address"`
	AddressID       int64  `json:"addressId" in:"path"`
	Recipient       string `json:"recipient"`
	Phone           string `json:"phone"`
	Country         string `json:"country"`
	Province        string `json:"province"`
	City            string `json:"city"`
	District        string `json:"district"`
	Detail          string `json:"detail"`
	PostalCode      string `json:"postalCode"`
	IsDefault       bool   `json:"isDefault"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type UpdateRes Item

type DeleteReq struct {
	g.Meta          `path:"/addresses/{addressId}" method:"delete" tags:"Identity-shop" summary:"Delete a customer address"`
	AddressID       int64  `json:"addressId" in:"path"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type DeleteRes struct {
	OK bool `json:"ok"`
}

type ReplaceDefaultReq struct {
	g.Meta          `path:"/addresses/default" method:"put" tags:"Identity-shop" summary:"Replace the default customer address"`
	AddressID       int64  `json:"addressId"`
	CommandKey      string `json:"commandKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type ReplaceDefaultRes Item
