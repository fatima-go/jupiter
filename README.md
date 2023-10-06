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