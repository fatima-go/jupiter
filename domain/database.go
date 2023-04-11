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
// @date 2018. 8. 20. AM 9:39
//

package domain

type SqlProvider interface {
	GetSqlFindAllDep() string
	GetSqlFindDepByPoint() string
	GetSqlFindDepByAddress(ipAddress string) string
	GetSqlFindSingleDep() string
	GetSqlFindDep() string
	GetSqlFindDepByEndpoint() string
	GetSqlDeleteDepByEndpoint() string
	GetSqlDeleteAllDep() string
	GetSqlInsertDep() string
	GetSqlDeleteOldToken() string
	GetSqlInsertToken() string
	GetSqlFindRoleFromToken() string
	GetSqlCountUserById() string
	GetSqlInsertUser() string
	GetSqlUpdateUser() string
	GetSqlFindUserById() string
	GetSqlDeleteUserById() string
	GetSqlCountAllUsers() string
}
