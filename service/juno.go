//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
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
// @date 2017. 2. 23. AM 10:55
//

package service

import (
	"strings"
	"sync"
	"throosea.com/fatima/lib"
	"throosea.com/jupiter/domain"
	"throosea.com/jupiter/web"
	"throosea.com/log"
	"time"
)

var mutex sync.Mutex

func (interactor *DomainInteractor) GetJunoEndpoint(point domain.PackagePoint, remoteAddr string) *domain.JunoPackage {
	juno := interactor.JunoRepository.FindByPoint(point)
	if juno == nil {
		juno = interactor.JunoRepository.FindByAddress(remoteAddr)
	}

	return juno
}

func (interactor *DomainInteractor) RegistJunoPackage(juno domain.JunoRegistration) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("try to REGIST juno registration : %s", juno)
	point := domain.PackagePoint{Host: juno.Host, Name: juno.Name}
	summary := interactor.JunoRepository.FindAll()
	element := summary.FindByPoint(point)
	if element != nil {
		log.Debug("update exist juno : %s", element)
		element.Endpoint = juno.Endpoint
		element.RegistDate = time.Now().Unix()
		element.Status = domain.JUNO_STATUS_ALIVE
		element.Platform = juno.Platform
		interactor.JunoRepository.SaveAll()
		return
	}

	log.Debug("regist new juno : %s", juno)
	juno.RegistDate = time.Now().Unix()
	juno.Status = domain.JUNO_STATUS_ALIVE
	interactor.JunoRepository.Save(juno)
}

func (interactor *DomainInteractor) UnregistJunoPackage(endpoint string) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("try to UNREGIST juno : %s", endpoint)

	summary := interactor.JunoRepository.FindAll()
	element := summary.FindByEndpoint(endpoint)
	if element != nil {
		log.Debug("update exist juno : %s", element)
		//element.Endpoint = juno.Endpoint
		//element.RegistDate = time.Now().Unix()
		element.Status = domain.JUNO_STATUS_DEAD
		interactor.JunoRepository.SaveAll()
		return
	}

	/*
		juno := interactor.JunoRepository.FindByEndpoint(endpoint)
		if juno == nil {
			log.Warn("Not found juno for endpoint : %s", endpoint)
			return
		}

		juno.Status = domain.JUNO_STATUS_DEAD
		interactor.JunoRepository.SaveAll()
	*/
}

func (interactor *DomainInteractor) RemoveJunoPackage(endpoint string) {
	log.Info("try to REMOVE juno : %s", endpoint)

	juno := interactor.JunoRepository.FindByEndpoint(endpoint)
	if juno == nil {
		log.Warn("Not found juno for endpoint : %s", endpoint)
		return
	}

	interactor.JunoRepository.Delete(endpoint)
}

func (interactor *DomainInteractor) GetPackageSummary(group string, location *time.Location) map[string]domain.PackageSummary {
	report := make(map[string]domain.PackageSummary)
	summary := domain.NewPackageSummary()
	hostMap := make(map[string]int)

	all := interactor.JunoRepository.FindAll()

	refreshPackageHealth(all)
	interactor.JunoRepository.SaveAll()

	if len(group) > 0 {
		log.Debug("retrieve group : %s", group)
		compName := strings.ToLower(group)
		for _, g := range all.Groups {
			if compName == strings.ToLower(g.Name) {
				summary.Deployment = append(summary.Deployment, g.Clone(location))
				summary.GroupCount = 1
				summary.PackageCount = len(g.Packages)
				for _, p := range g.Packages {
					hostMap[strings.ToLower(p.Host)]++
				}
				break
			}
		}
	} else {
		log.Debug("retrieve all groups")
		for _, g := range all.Groups {
			summary.Deployment = append(summary.Deployment, g.Clone(location))
			summary.PackageCount = summary.PackageCount + len(g.Packages)
			for _, p := range g.Packages {
				hostMap[strings.ToLower(p.Host)]++
			}
		}
		summary.GroupCount = len(all.Groups)
	}

	for _, v := range hostMap {
		summary.HostCount = summary.HostCount + v
	}

	report["summary"] = summary
	return report
}

func refreshPackageHealth(summary *domain.JunoSummary) {
	size := 0
	for _, g := range summary.Groups {
		size = size + len(g.Packages)
	}

	if size == 0 {
		return
	}

	log.Debug("check package health")
	cyBarrier := lib.NewCyclicBarrier(size, func() { log.Debug("finish refreshing juno health") })
	for i := 0; i < len(summary.Groups); i++ {
		group := &summary.Groups[i]
		for j := 0; j < len(group.Packages); j++ {
			pack := &group.Packages[j]
			cyBarrier.Dispatch(func() { callHealthCheck(pack) })
		}
	}
	cyBarrier.Wait()
}

func callHealthCheck(pack *domain.JunoPackage) bool {
	httpClient := web.NewHttpClient(nil)
	_, err := httpClient.Post(buildRestUrl(pack.Endpoint, "/package/health/v1"), nil)
	if err != nil {
		log.Warn("fail to check health for %s[%s] : %s", pack.Name, pack.Host, err.Error())
		pack.Status = domain.JUNO_STATUS_DEAD
	} else {
		pack.Status = domain.JUNO_STATUS_ALIVE
	}

	return true
}

func buildRestUrl(endpoint string, suffix string) string {
	var url string
	if endpoint[len(endpoint)-1] == '/' {
		if suffix[0] == '/' {
			url = endpoint + suffix[1:]
		} else {
			url = endpoint + suffix
		}
	} else {
		if suffix[0] == '/' {
			url = endpoint + suffix
		} else {
			url = endpoint + "/" + suffix
		}
	}
	return url
}
