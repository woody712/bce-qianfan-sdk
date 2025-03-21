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
	"sync"

	"github.com/spf13/viper"
)

// 默认配置
var defaultConfig = map[string]string{
	"QIANFAN_AK":                                  "",
	"QIANFAN_SK":                                  "",
	"QIANFAN_ACCESS_KEY":                          "",
	"QIANFAN_SECRET_KEY":                          "",
	"QIANFAN_BEARER_TOKEN":                        "",
	"QIANFAN_BASE_URL":                            "https://aip.baidubce.com",
	"QIANFAN_IAM_SIGN_EXPIRATION_SEC":             "300",
	"QIANFAN_BEARER_TOKEN_EXPIRATION_SEC":         "3600",
	"QIANFAN_BEARER_TOKEN_REFRESH_ADVANCE":        "60",
	"QIANFAN_CONSOLE_BASE_URL":                    "https://qianfan.baidubce.com",
	"QIANFAN_IAM_BASE_URL":                        "http://iam.bj.baidubce.com",
	"QIANFAN_ACCESS_TOKEN_REFRESH_MIN_INTERVAL":   "3600",
	"QIANFAN_LLM_API_RETRY_COUNT":                 "1",
	"QIANFAN_LLM_API_RETRY_BACKOFF_FACTOR":        "0",
	"QIANFAN_LLM_API_RETRY_TIMEOUT":               "0",
	"QIANFAN_INFER_RESOURCE_REFRESH_MIN_INTERVAL": "600",
}

// SDK 使用的全局配置，可以用 GetConfig() 获取
type Config struct {
	AK                            string  `mapstructure:"QIANFAN_AK"`
	SK                            string  `mapstructure:"QIANFAN_SK"`
	ApiKey                        string  `mapstructure:"QIANFAN_API_KEY"`
	AccessKey                     string  `mapstructure:"QIANFAN_ACCESS_KEY"`
	SecretKey                     string  `mapstructure:"QIANFAN_SECRET_KEY"`
	AccessToken                   string  `mapstructure:"QIANFAN_ACCESS_TOKEN"`
	BearerToken                   string  `mapstructure:"QIANFAN_BEARER_TOKEN"`
	BaseURL                       string  `mapstructure:"QIANFAN_BASE_URL"`
	IAMSignExpirationSeconds      int     `mapstructure:"QIANFAN_IAM_SIGN_EXPIRATION_SEC"`
	BearerTokenExpirationSeconds  int     `mapstructure:"QIANFAN_BEARER_TOKEN_EXPIRATION_SEC"`
	BearerTokenRefreshAdvance     int     `mapstructure:"QIANFAN_BEARER_TOKEN_REFRESH_ADVANCE"`
	ConsoleBaseURL                string  `mapstructure:"QIANFAN_CONSOLE_BASE_URL"`
	IAMBaseURL                    string  `mapstructure:"QIANFAN_IAM_BASE_URL"`
	AccessTokenRefreshMinInterval int     `mapstructure:"QIANFAN_ACCESS_TOKEN_REFRESH_MIN_INTERVAL"`
	LLMRetryCount                 int     `mapstructure:"QIANFAN_LLM_API_RETRY_COUNT"`
	LLMRetryTimeout               float32 `mapstructure:"QIANFAN_LLM_API_RETRY_TIMEOUT"`
	LLMRetryBackoffFactor         float32 `mapstructure:"QIANFAN_LLM_API_RETRY_BACKOFF_FACTOR"`
	InferResourceRefreshInterval  int     `mapstructure:"QIANFAN_INFER_RESOURCE_REFRESH_MIN_INTERVAL"`
	RetryErrCodes                 []int
}

func setConfigDefaultValue(vConfig *viper.Viper) {
	// 因为 viper 自动绑定无法在 unmarshal 时使用，所以这里要手动设置默认值
	for k, v := range defaultConfig {
		vConfig.SetDefault(k, v)
	}
}

func loadConfigFromEnv() *Config {
	vConfig := viper.New()

	vConfig.SetConfigFile(".env")
	vConfig.SetConfigType("dotenv")
	vConfig.AutomaticEnv()
	setConfigDefaultValue(vConfig)

	// ignore error if config file not found
	_ = vConfig.ReadInConfig()

	config := &Config{}
	if err := vConfig.Unmarshal(&config); err != nil {
		logger.Panicf("load config file failed with error `%v`, please check your config.", err)
	}
	return config
}

var _config *Config = nil
var _configInitOnce sync.Once

// 获取全局配置，可以通过如下方式修改配置
// 可以在代码中手动设置 `AccessKey` 和 `SecretKey`，具体如下：
//
//	qianfan.GetConfig().AccessKey = "your_access_key"
//	qianfan.GetConfig().SecretKey = "your_secret_key"
func GetConfig() *Config {
	_configInitOnce.Do(func() {
		_config = loadConfigFromEnv()
		_config.RetryErrCodes = []int{
			ServiceUnavailableErrCode,
			ServerHighLoadErrCode,
			QPSLimitReachedErrCode,
			RPMLimitReachedErrCode,
			TPMLimitReachedErrCode,
			AppNotExistErrCode,
		}
	})
	return _config
}
