//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with server work for additional information
// regarding copyright ownership.  The ASF licenses server file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use server file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project jupiter
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 2. 22. PM 2:10
//

package engine

import (
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/jupiter/service"
	"github.com/fatima-go/jupiter/web"
	"github.com/fatima-go/jupiter/web/v1"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	"time"
)

const (
	PropWebServerAddress = "webserver.address"
	PropWebServerPort    = "webserver.port"
)

func NewWebServer(fatimaRuntime fatima.FatimaRuntime) *JupiterHttpServer {
	server := new(JupiterHttpServer)
	server.fatimaRuntime = fatimaRuntime
	return server
}

type JupiterHttpServer struct {
	fatimaRuntime fatima.FatimaRuntime
	webService    *web.WebService
	router        *mux.Router
	loggingRouter http.Handler
	listenAddress string
}

func (server *JupiterHttpServer) Initialize() bool {
	log.Info("JupiterHttpServer Initialize()")

	v, ok := server.fatimaRuntime.GetConfig().GetValue(PropWebServerAddress)
	if !ok {
		v = ""
	}
	server.listenAddress = v

	v, ok = server.fatimaRuntime.GetConfig().GetValue(PropWebServerPort)
	if !ok {
		v = "9190"
	}
	server.listenAddress = fmt.Sprintf("%s:%s", server.listenAddress, v)
	log.Info("web server listen : %s", server.listenAddress)

	domainInteractor, err := service.NewDomainInteractor(server.fatimaRuntime)
	if err != nil {
		log.Warn("fail to create interactor  %s", err.Error())
		return false
	}

	server.webService = web.GetWebService()
	server.webService.Regist(v1.NewWebService(domainInteractor))

	server.router = mux.NewRouter().StrictSlash(true)
	server.webService.GenerateSubRouter(server.router)
	server.loggingRouter = handlers.LoggingHandler(server, server.router)

	return true
}

func (server *JupiterHttpServer) Write(p []byte) (n int, err error) {
	server.access(string(p[:len(p)-1]))
	return len(p), nil
}

func (server *JupiterHttpServer) access(access string) {
	if len(access) < 10 {
		return
	}

	remote := strings.Split(access, " ")[0]
	idx := strings.Index(access, " /")
	if idx < 1 {
		return
	}
	uri := strings.Split(access[idx+1:], " ")[0]
	log.Info("%s -> %s", remote, uri)
}

func (server *JupiterHttpServer) Bootup() {
	log.Info("JupiterHttpServer Bootup()")
}

func (server *JupiterHttpServer) Shutdown() {
	log.Info("JupiterHttpServer Shutdown()")
}

func (server *JupiterHttpServer) GetType() fatima.FatimaComponentType {
	return fatima.COMP_READER
}

func (server *JupiterHttpServer) StartListening() {
	log.Info("called JupiterHttpServer StartListening()")

	srv := &http.Server{
		Handler:      server.loggingRouter,
		Addr:         server.listenAddress,
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  60 * time.Second,
	}

	log.Info("start web server listening...")
	err := srv.ListenAndServe()
	if err != nil {
		log.Error("fail to start web server : %s", err.Error())
		server.fatimaRuntime.Stop()
	}

}
