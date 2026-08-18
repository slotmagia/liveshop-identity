package authendpoint

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/lvtuopen-ai/kernel-go/accessidentity"
	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/middleware"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

type Endpoint struct {
	auth       *biz.Authentication
	directory  *biz.Directory
	issuer     *accessidentity.Issuer
	verifier   *accessidentity.Verifier
	settings   config.AccessIdentity
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func New(auth *biz.Authentication, directory *biz.Directory, issuer *accessidentity.Issuer, verifier *accessidentity.Verifier, settings config.AccessIdentity) (*Endpoint, error) {
	accessTTL, err := settings.AccessDuration()
	if err != nil {
		return nil, err
	}
	refreshTTL, err := settings.RefreshDuration()
	if err != nil {
		return nil, err
	}
	if auth == nil || directory == nil || issuer == nil || verifier == nil {
		return nil, model.ErrUnavailable
	}
	return &Endpoint{auth: auth, directory: directory, issuer: issuer, verifier: verifier, settings: settings, accessTTL: accessTTL, refreshTTL: refreshTTL}, nil
}

func Register(root *ghttp.RouterGroup, endpoint *Endpoint) {
	root.Group("/auth", func(group *ghttp.RouterGroup) {
		group.Middleware(web.ResponseHandler)
		group.Middleware(middleware.RequestMetadata)
		group.Bind(endpoint)
	})
}

type LoginReq struct {
	g.Meta   `path:"/login" method:"post" tags:"Identity-auth" summary:"Authenticate a principal"`
	Realm    string `json:"realm" v:"required"`
	Username string `json:"username" v:"required"`
	Password string `json:"password" v:"required"`
	ShopCode string `json:"shopCode"`
}
type LoginRes struct {
	AccessToken string    `json:"accessToken"`
	ExpiresIn   int64     `json:"expiresIn"`
	Principal   Principal `json:"principal"`
}

type GuestReq struct {
	g.Meta   `path:"/guest" method:"post" tags:"Identity-auth" summary:"Create a shop-bound guest session"`
	ShopCode string `json:"shopCode" v:"required"`
}
type GuestRes = LoginRes

type RefreshReq struct {
	g.Meta `path:"/refresh" method:"post" tags:"Identity-auth" summary:"Rotate refresh token"`
}
type RefreshRes = LoginRes
type LogoutReq struct {
	g.Meta `path:"/logout" method:"post" tags:"Identity-auth" summary:"Revoke current session"`
}
type LogoutRes struct {
	Revoked bool `json:"revoked"`
}
type MeReq struct {
	g.Meta `path:"/me" method:"get" tags:"Identity-auth" summary:"Read current principal"`
}
type MeRes struct {
	Principal Principal `json:"principal"`
	Context   Context   `json:"context"`
}
type SwitchReq struct {
	g.Meta         `path:"/context/switch" method:"post" tags:"Identity-auth" summary:"Switch verified context"`
	OrganizationID int64 `json:"organizationId"`
	MerchantID     int64 `json:"merchantId"`
	ShopID         int64 `json:"shopId"`
}
type SwitchRes = LoginRes

type Principal struct {
	Realm          string `json:"realm"`
	PrincipalType  string `json:"principalType"`
	Subject        string `json:"subject"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	OrganizationID int64  `json:"organizationId,omitempty"`
	MerchantID     int64  `json:"merchantId,omitempty"`
}
type Context struct {
	OrganizationID  int64  `json:"organizationId,omitempty"`
	MerchantID      int64  `json:"merchantId,omitempty"`
	ShopID          int64  `json:"shopId,omitempty"`
	ContextVersion  uint64 `json:"contextVersion"`
	IdentityVersion uint64 `json:"identityVersion"`
}

func (e *Endpoint) Login(ctx context.Context, request *LoginReq) (*LoginRes, error) {
	realm, ok := principal.ParseRealm(strings.ToUpper(strings.TrimSpace(request.Realm)))
	if !ok {
		return nil, unauthorized()
	}
	if !realmMatchesSurface(realm, requestFrom(ctx).Header.Get("X-Liveshop-Surface")) {
		return nil, unauthorized()
	}
	refresh := secureToken()
	refreshHash := sha256.Sum256([]byte(refresh))
	now := time.Now()
	result, err := e.auth.Login(ctx, biz.LoginCommand{Realm: realm, Username: request.Username, Password: request.Password,
		ShopCode:  request.ShopCode,
		SessionID: secureToken(), FamilyID: secureToken(), RefreshHash: refreshHash, ExpiresAt: now.Add(e.refreshTTL),
		IPAddress: requestFrom(ctx).RemoteAddr, UserAgent: requestFrom(ctx).UserAgent()})
	if err != nil {
		return nil, authFailure(err)
	}
	e.setRefreshCookie(requestFrom(ctx), realm, refresh, now.Add(e.refreshTTL))
	return e.loginResponse(result, request.Username)
}

func (e *Endpoint) Guest(ctx context.Context, request *GuestReq) (*GuestRes, error) {
	requestContext := requestFrom(ctx)
	if !guestSurface(requestContext.Header.Get("X-Liveshop-Surface")) {
		return nil, unauthorized()
	}
	refresh := secureToken()
	refreshHash := sha256.Sum256([]byte(refresh))
	now := time.Now()
	result, err := e.auth.Guest(ctx, biz.GuestCommand{
		Subject: "guest-" + secureToken(), ShopCode: strings.TrimSpace(request.ShopCode),
		SessionID: secureToken(), FamilyID: secureToken(), RefreshHash: refreshHash, ExpiresAt: now.Add(e.refreshTTL),
		IPAddress: requestContext.RemoteAddr, UserAgent: requestContext.UserAgent(),
	})
	if err != nil {
		return nil, authFailure(err)
	}
	e.setRefreshCookie(requestContext, principal.RealmCustomer, refresh, now.Add(e.refreshTTL))
	return e.loginResponse(result, "")
}

func (e *Endpoint) Refresh(ctx context.Context, _ *RefreshReq) (*RefreshRes, error) {
	request := requestFrom(ctx)
	realm, ok := realmForSurface(request.Header.Get("X-Liveshop-Surface"))
	if !ok {
		return nil, unauthorized()
	}
	refresh := request.Cookie.Get(e.cookieName(realm)).String()
	result, rotated, err := e.auth.RotateRefresh(ctx, realm, refresh, time.Now().Add(e.refreshTTL))
	if err != nil {
		return nil, authFailure(err)
	}
	if result.Subject.Realm != realm {
		return nil, unauthorized()
	}
	e.setRefreshCookie(request, realm, rotated, time.Now().Add(e.refreshTTL))
	return e.loginResponse(result, "")
}

func (e *Endpoint) Logout(ctx context.Context, _ *LogoutReq) (*LogoutRes, error) {
	request := requestFrom(ctx)
	realm, ok := realmForSurface(request.Header.Get("X-Liveshop-Surface"))
	if !ok {
		return nil, unauthorized()
	}
	refresh := request.Cookie.Get(e.cookieName(realm)).String()
	if err := e.auth.Logout(ctx, realm, refresh); err != nil {
		return nil, web.Failure(err)
	}
	e.setRefreshCookie(request, realm, "", time.Unix(1, 0))
	return &LogoutRes{Revoked: true}, nil
}

func (e *Endpoint) Me(ctx context.Context, _ *MeReq) (*MeRes, error) {
	claims, err := e.accessClaims(requestFrom(ctx))
	if err != nil {
		return nil, unauthorized()
	}
	return &MeRes{Principal: principalFromClaims(claims, ""), Context: contextFromClaims(claims)}, nil
}

func (e *Endpoint) Switch(ctx context.Context, request *SwitchReq) (*SwitchRes, error) {
	claims, err := e.accessClaims(requestFrom(ctx))
	if err != nil {
		return nil, unauthorized()
	}
	selected := model.SelectedContext{OrganizationID: request.OrganizationID, ShopContext: model.ShopContext{MerchantID: request.MerchantID, ShopID: request.ShopID}}
	result, err := e.auth.SwitchContext(ctx, biz.SwitchContextCommand{SessionID: claims.SessionID, Subject: claims.Subject, Selected: selected, ExpectedContextVersion: claims.ContextVersion})
	if err != nil {
		return nil, web.Failure(err)
	}
	return e.loginResponse(result, "")
}

func (e *Endpoint) loginResponse(session biz.AuthenticatedSession, username string) (*LoginRes, error) {
	claims := accessidentity.Claims{Subject: session.Subject.ID, Realm: session.Subject.Realm, PrincipalType: session.Subject.PrincipalType,
		SessionID: session.SessionID, OrganizationID: session.Organization.ID, MerchantID: session.Selected.MerchantID,
		ShopID:         session.Selected.ShopID,
		ContextVersion: session.ContextVersion, IdentityVersion: session.Subject.Version}
	if claims.MerchantID == 0 {
		claims.MerchantID = session.Member.MerchantID
	}
	token, err := e.issuer.Sign(claims, e.accessTTL)
	if err != nil {
		return nil, web.Failure(err)
	}
	return &LoginRes{AccessToken: token, ExpiresIn: int64(e.accessTTL.Seconds()), Principal: Principal{Realm: session.Subject.Realm.String(), PrincipalType: session.Subject.PrincipalType.String(), Subject: session.Subject.ID, Username: username, DisplayName: session.Subject.DisplayName, OrganizationID: session.Organization.ID, MerchantID: claims.MerchantID}}, nil
}

func (e *Endpoint) accessClaims(request *ghttp.Request) (accessidentity.Claims, error) {
	token, ok := accessidentity.Bearer(request.Header.Get("Authorization"))
	if !ok {
		return accessidentity.Claims{}, errors.New("missing token")
	}
	return e.verifier.Verify(token)
}

func (e *Endpoint) setRefreshCookie(request *ghttp.Request, realm principal.Realm, value string, expires time.Time) {
	cookie := (&http.Cookie{Name: e.cookieName(realm), Value: value, Path: "/auth", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: e.settings.CookieSecure, SameSite: http.SameSiteLaxMode}).String()
	request.Response.Header().Add("Set-Cookie", cookie)
}

func (e *Endpoint) cookieName(realm principal.Realm) string {
	switch realm {
	case principal.RealmPlatform:
		return e.settings.PlatformRefreshCookie
	case principal.RealmMerchant:
		return e.settings.MerchantRefreshCookie
	case principal.RealmCustomer:
		return e.settings.CustomerRefreshCookie
	default:
		return ""
	}
}

func realmForSurface(surface string) (principal.Realm, bool) {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "admin":
		return principal.RealmPlatform, true
	case "merch":
		return principal.RealmMerchant, true
	case "shop", "live":
		return principal.RealmCustomer, true
	default:
		return "", false
	}
}

func realmMatchesSurface(realm principal.Realm, surface string) bool {
	selected, ok := realmForSurface(surface)
	return ok && selected == realm
}

func guestSurface(surface string) bool {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "shop", "live":
		return true
	default:
		return false
	}
}

func principalFromClaims(claims accessidentity.Claims, username string) Principal {
	return Principal{Realm: claims.Realm.String(), PrincipalType: claims.PrincipalType.String(), Subject: claims.Subject, Username: username, OrganizationID: claims.OrganizationID, MerchantID: claims.MerchantID}
}
func contextFromClaims(claims accessidentity.Claims) Context {
	return Context{OrganizationID: claims.OrganizationID, MerchantID: claims.MerchantID, ShopID: claims.ShopID, ContextVersion: claims.ContextVersion, IdentityVersion: claims.IdentityVersion}
}
func secureToken() string {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
func unauthorized() error {
	return &web.HTTPError{Status: http.StatusUnauthorized, Cause: biz.ErrInvalidCredentials}
}
func authFailure(err error) error {
	if errors.Is(err, biz.ErrInvalidCredentials) {
		return unauthorized()
	}
	return web.Failure(err)
}

// GoFrame stores the active request in the handler context.
func requestFrom(ctx context.Context) *ghttp.Request { return g.RequestFromCtx(ctx) }
