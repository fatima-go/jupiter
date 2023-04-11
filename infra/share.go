//
// Copyright (c) 2018 SK TECHX.
// All right reserved.
//
// This software is the confidential and proprietary information of SK TECHX.
// You shall not disclose such Confidential Information and
// shall use it only in accordance with the terms of the license agreement
// you entered into with SK TECHX.
//
//
// @project jupiter
// @author 1100282
// @date 2018. 8. 20. AM 9:05
//

package infra

import (
	"strings"
)

func ExtractIpAddress(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[:idx]
	}
	return addr
}
