package main

const usage = `
# include
include-config     filename none

# general
root               filename none # also as default parameter, when not set then serving stdio
cachedir           filename none
max-search-results int      0 # search disabled default

# http
address            string   :9090 # when filename, then unix socket
tls-key            string   automatic when listening on http and tls-key-file not defined
tls-cert           string   automatic when listening on http and tls-key-file not defined
tls-key-file       filename none
tls-cert-file      filename none
allow-cookies      bool     false
max-request-body   int      none
max-request-header int      1<<20
proxy              json     none # e.g. [{"method": "AUTH", "address": "/var/sockets/auth-socket"}]
proxy-file         filename none

# auth
authenticate       bool     false
public-user        username none # when not set and auth enabled then no public access
aes-key            string   automatic when auth enabled and aes-key-file not defined
aes-iv             string   automatic when auth enabled and aes-cert-file not defined
aes-key-file       filename none
aes-iv-file        filename none
token-validity     seconds  60 * 60 * 24 * 80
max-user-processes int      unlimited
process-idle-time  seconds  360
`
