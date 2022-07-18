// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package common_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dell/csm-metrics-powerscale/internal/common"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func Test_Run(t *testing.T) {

	tests := map[string]func(t *testing.T) (filePath string, expectError bool){
		"success": func(*testing.T) (string, bool) {
			viper.SetDefault("POWERSCALE_ISICLIENT_VERBOSE", 0)
			viper.SetDefault("POWERSCALE_ISICLIENT_AUTH_TYPE", 0)
			viper.SetDefault("POWERSCALE_ISICLIENT_INSECURE", true)
			return "testdata/sample-config.yaml", false
		},
		"success with default params": func(*testing.T) (string, bool) {
			viper.SetDefault("POWERSCALE_ISICLIENT_VERBOSE", 12)
			viper.SetDefault("POWERSCALE_ISICLIENT_AUTH_TYPE", 12)
			viper.SetDefault("POWERSCALE_ISICLIENT_INSECURE", "wrong")
			return "testdata/sample-config-default.yaml", false
		},
		"file format": func(*testing.T) (string, bool) {
			return "testdata/invalid-format.yaml", true
		},
		"no cluster name": func(*testing.T) (string, bool) {
			return "testdata/no-cluster-name.yaml", true
		},
		"connection failed": func(*testing.T) (string, bool) {
			return "testdata/connection-failed.yaml", true
		},
	}

	handler := getHandler()
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	urls := strings.Split(strings.TrimPrefix(server.URL, "https://"), ":")
	serverIP := urls[0]
	serverPort := urls[1]

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := logrus.New()
			filePath, expectError := test(t)

			fileContentBytes, _ := ioutil.ReadFile(filePath)

			newContent := strings.Replace(string(fileContentBytes), "[serverip]", serverIP, 1)
			newContent = strings.Replace(newContent, "[serverport]", serverPort, 1)
			ioutil.WriteFile(filePath, []byte(newContent), 0644)

			clusters, defaultCluster, err := common.GetPowerScaleClusters(filePath, logger)

			if expectError {
				assert.Nil(t, clusters)
				assert.Nil(t, defaultCluster)
				assert.NotNil(t, err)
			} else {
				assert.NotNil(t, clusters)
				assert.NotNil(t, defaultCluster)
				assert.Nil(t, err)
			}
			ioutil.WriteFile(filePath, fileContentBytes, 0644)
		})
	}
}

var isilonRouter http.Handler

// getFileHandler returns an http.Handler that
func getHandler() http.Handler {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("handler called: %s %s", r.Method, r.URL)
			if isilonRouter == nil {
				getRouter().ServeHTTP(w, r)
			}
		})

	return handler
}

func getRouter() http.Handler {
	isilonRouter := mux.NewRouter()
	isilonRouter.HandleFunc("/platform/latest/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{\"latest\": \"14\"}"))
	})
	return isilonRouter
}
