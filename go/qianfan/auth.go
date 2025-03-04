// Copyright (c) 2024 Baidu, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package qianfan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
)

type AccessTokenRequest struct {
	GrantType    string `mapstructure:"grant_type"`
	ClientId     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

func newAccessTokenRequest(ak, sk string) *AccessTokenRequest {
	return &AccessTokenRequest{
		GrantType:    "client_credentials",
		ClientId:     ak,
		ClientSecret: sk,
	}
}

type AccessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	SessionKey       string `json:"session_key"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"`
	SessionSecret    string `json:"session_secret"`
	baseResponse
}

func (r *AccessTokenResponse) GetErrorCode() string {
	return r.Error
}

type credential struct {
	AK string
	SK string
}

type accessToken struct {
	token         string
	lastUpateTime time.Time
}

type AuthManager struct {
	tokenMap map[credential]*accessToken
	lock     sync.Mutex
	*Requestor
}

func maskAk(ak string) string {
	unmaskLen := 6
	if len(ak) < unmaskLen {
		return ak
	}
	return fmt.Sprintf("%s******", ak[:unmaskLen])
}

var _authManager *AuthManager
var _authManagerInitOnce sync.Once

func GetAuthManager() *AuthManager {
	_authManagerInitOnce.Do(func() {
		_authManager = &AuthManager{
			tokenMap:  make(map[credential]*accessToken),
			lock:      sync.Mutex{},
			Requestor: newRequestor(makeOptions()),
		}
	})
	return _authManager
}

func (m *AuthManager) GetAccessToken(ctx context.Context, ak, sk string) (string, error) {
	token, ok := func() (*accessToken, bool) {
		m.lock.Lock()
		defer m.lock.Unlock()
		token, ok := m.tokenMap[credential{ak, sk}]
		return token, ok
	}()
	if ok {
		return token.token, nil
	}
	logger.Infof("Access token of ak `%s` not found, tring to refresh it...", maskAk(ak))
	return m.GetAccessTokenWithRefresh(ctx, ak, sk)
}

func (m *AuthManager) GetAccessTokenWithRefresh(ctx context.Context, ak, sk string) (string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	token, ok := m.tokenMap[credential{ak, sk}]
	if ok {
		lastUpdate := token.lastUpateTime
		current := time.Now()
		// 最近更新时间小于最小刷新间隔，则直接返回
		// 避免多个请求同时刷新，导致token被刷新多次
		if current.Sub(lastUpdate) < time.Duration(GetConfig().AccessTokenRefreshMinInterval)*time.Second {
			logger.Debugf("Access token of ak `%s` was freshed %s ago, skip refreshing", maskAk(ak), current.Sub(lastUpdate))
			return token.token, nil
		}
	}

	resp := AccessTokenResponse{}
	req, err := newAuthRequest("POST", authAPIPrefix, nil)
	if err != nil {
		return "", err
	}
	params := newAccessTokenRequest(ak, sk)
	paramsMap := make(map[string]string)
	err = mapstructure.Decode(params, &paramsMap)
	if err != nil {
		return "", err
	}
	logger.Infof("paramsMap: %v", paramsMap)
	req.Params = paramsMap
	err = m.Requestor.request(ctx, req, &resp)
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		logger.Errorf("refresh access token of ak `%s` failed with error: %s", maskAk(ak), resp.ErrorDescription)
		return "", &APIError{Msg: resp.ErrorDescription}
	}
	logger.Infof("Access token of ak `%s` was refreshed", maskAk(ak))
	m.tokenMap[credential{ak, sk}] = &accessToken{
		token:         resp.AccessToken,
		lastUpateTime: time.Now(),
	}
	GetConfig().AccessToken = resp.AccessToken
	return resp.AccessToken, nil
}

type IAMBearerTokenResponse struct {
	UserID     string `json:"userId"`
	Token      string `json:"token"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
	ExpireTime string `json:"expireTime"`
	baseResponse
}

func (r *IAMBearerTokenResponse) GetErrorCode() string {
	return "Get IAM Bearer Token Error"
}

func GetBearerToken() (string, error) {
	return GetBearerTokenManager().GetAccessTokenWithRefresh()
}

type BearerToken struct {
	token          string
	ExpireTime     time.Time
	ExpireTimeBuff time.Duration
}

var _bearerTokenManager *BearerTokenManager
var _bearerTokenManagerInitOnce sync.Once

type BearerTokenManager struct {
	token    *BearerToken
	lock     sync.Mutex
	isPreset bool
	*Requestor
}

func GetBearerTokenManager() *BearerTokenManager {
	_bearerTokenManagerInitOnce.Do(func() {
		_bearerTokenManager = &BearerTokenManager{
			lock:      sync.Mutex{},
			Requestor: newRequestor(makeOptions()),
		}

		token := GetConfig().BearerToken
		if token != "" {
			_bearerTokenManager.token = &BearerToken{
				token: token,
			}
			_bearerTokenManager.isPreset = true
		} else {
			_bearerTokenManager.isPreset = false
		}
	})
	return _bearerTokenManager
}

func (m *BearerTokenManager) GetAccessTokenWithRefresh() (string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	token := m.token

	if token != nil && (m.isPreset || time.Until(token.ExpireTime) > token.ExpireTimeBuff) {
		return token.token, nil
	}

	resp := IAMBearerTokenResponse{}
	refreshPath := "/v1/BCE-BEARER/token"
	if GetConfig().BearerTokenExpirationSeconds > 0 {
		refreshPath += fmt.Sprintf("?expireInSeconds=%d", GetConfig().BearerTokenExpirationSeconds)
	}
	req, err := NewIAMBearerTokenRequest(
		"GET",
		refreshPath,
		nil,
	)
	if err != nil {
		return "", err
	}

	err = newRequestor(makeOptions()).request(context.TODO(), req, &resp)
	if err != nil {
		return "", err
	}
	logger.Info("Get IAM Bearer Token Success")

	expireTime, err := time.Parse(time.RFC3339, resp.ExpireTime)
	if err != nil {
		logger.Errorf("Parse IAM Bearer Token Expire Time Failed: %s", err)
		return "", err
	}

	token = &BearerToken{
		token:          resp.Token,
		ExpireTime:     expireTime,
		ExpireTimeBuff: time.Duration(GetConfig().BearerTokenRefreshAdvance) * time.Second,
	}

	m.token = token
	GetConfig().BearerToken = token.token
	return token.token, nil
}
