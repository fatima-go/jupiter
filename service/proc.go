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
// @date 2017. 3. 4. PM 6:51
//

package service

import (
	"github.com/fatima-go/jupiter/domain"
	"strings"
)

func (interactor *DomainInteractor) GetEndpointList(groupName string, point domain.PackagePoint, address string) []string {
	list := make([]string, 0)

	if len(groupName) > 0 {
		summary := interactor.JunoRepository.FindAll()
		return retrieveByGroup(summary, groupName)
	} else if !point.IsEmpty() {
		juno := interactor.JunoRepository.FindByPoint(point)
		if juno != nil {
			list = append(list, juno.Endpoint)
		}
	} else {
		juno := interactor.JunoRepository.FindByAddress(address)
		if juno != nil {
			list = append(list, juno.Endpoint)
		}
	}

	return list
}

func retrieveByGroup(summary *domain.JunoSummary, groupName string) []string {
	list := make([]string, 0)

	compName := strings.ToLower(groupName)
	for _, g := range summary.Groups {
		if compName == strings.ToLower(g.Name) {
			for _, p := range g.Packages {
				list = append(list, p.Endpoint)
			}
			break
		}
	}

	return list
}
