//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with r work for additional information
// regarding copyright ownership.  The ASF licenses r file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use r file except in compliance
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
// @date 2017. 2. 22. AM 11:31
//

package domain

import (
	"strings"
)

type PackagePoint struct {
	Host string
	Name string
}

func (pp PackagePoint) IsEmpty() bool {
	return pp.Host == ""
}

func NewPackagePoint(pack string) PackagePoint {
	data := PackagePoint{Name: "default"}
	if len(pack) < 1 {
		return data
	}

	i := strings.Index(pack, ":")
	if i < 0 {
		data.Host = pack
		return data
	}
	data.Host = pack[:i]
	data.Name = pack[i+1 : len(pack)]
	return data
}

type PackageSummary struct {
	Deployment   []JunoGroup `json:"deployment"`
	GroupCount   int         `json:"group_count"`
	HostCount    int         `json:"host_count"`
	PackageCount int         `json:"package_count"`
}

func NewPackageSummary() PackageSummary {
	summary := PackageSummary{}
	summary.Deployment = make([]JunoGroup, 0)
	return summary
}

/*
{
	"summary": {
		"deployment": [
			{
				"group_name": "basic",
				"packages": [
					{
						"endpoint": "http://172.21.85.73:9180/9yaQqdax/",
						"host": "xfp-stg",
						"name": "default",
						"regist_date": "2016-12-02 11:38:14",
						"status": "ALIVE"
					}
				]
			}
		],
		"group_count": 1,
		"host_count": 1,
		"package_count": 1
	}
}
*/
