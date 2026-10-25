# jupiter #

jupiter process is ...

# application properties #

You can use juno properties like below.

name     | type   | default   | remark
---------:|:-------|:----------| :-----
webserver.address | string | 0.0.0.0   | jupiter listen ip address
webserver.port | int    | 9180      | jupiter listen port
auth  | string | basic     | authentication method (basic, ldap)
auth.ldap.helper.ip  | string | 127.0.0.1 | available when auth=ldap. ldap server ip address
auth.ldap.helper.port  | int    |           | available when auth=ldap. ldap server port
token.duration.seconds  | int    | 3600      | token duration(expire) seconds
token.duration.instant.seconds  | int    | 10        | instant token duration(expire) seconds
repo  | string    | memory    | user repository method. (memory, file)

# login profile #

jupiter 는 로그인 응답에 자신의 실행 환경 profile(`FATIMA_PROFILE`, 미설정 시 빈 값)을 담는다.
jupiter 1개는 하나의 환경(profile)만 담당한다는 전제이며, fatima-cmd 는 이 값으로 배포할 FAR 를 환경에 맞게 재조립한다.

- legacy `/auth/login/v1` : 응답 JSON 의 `profile` 키 (항상 포함)
- v2 `Identity.Login` : `Session.profile`. 빈 값은 proto3 에서 전송되지 않으므로 capability feature `session_profile` 로 지원 여부를 판단한다