---
date: 2016-11-23T01:07:31+00:00
title: Deployment
---

## Configuration file

Skydive is based on an unique binary and configuration file for the Agent and Analyzer.
Each Agent and Analyzer have his own section.

A configuration example can be found (here)[https://github.com/skydive-project/skydive/blob/master/etc/skydive.yml.default]

## Security

To secure communication between Agent(s) and Analyzer, Skydive relies on TLS communication with strict cross validation.
TLS communication can be enabled by defining X509 certificates in their respective section in the configuration file, like :

```
analyzer:
  X509_cert: /etc/ssl/certs/analyzer.domain.com.crt
  X509_key:  /etc/ssl/certs/analyzer.domain.com.key

agent:
  X509_cert: /etc/ssl/certs/agent.domain.com.crt
  X509_key:  /etc/ssl/certs/agent.domain.com.key
```

### Generate the certificates

### Certificate Signing Request (CSR)
```
openssl genrsa -out analyzer/analyzer.domain.com.key 2048
chmod 400 analyzer/analyzer.domain.com.key
openssl req -new -key analyzer/analyzer.domain.com.key -out analyzer/analyzer.domain.com.csr -subj "/CN=skydive-analyzer" -config skydive-openssl.cnf
```

### Analyzer (Server certificate CRT)
```
yes '' | openssl x509 -req -days 365  -signkey analyzer/analyzer.domain.com.key -in analyzer/analyzer.domain.com.csr -out analyzer/analyzer.domain.com.crt -extfile skydive-openssl.cnf -extensions v3_req
chmod 444 analyzer/analyzer.domain.com.crt
```

### Agent (Client certificate CRT)
```
openssl genrsa -out agent/agent.domain.com.key 2048
chmod 400 agent/agent.domain.com.key
yes '' | openssl req -new -key agent/agent.domain.com.key -out agent/agent.domain.com.csr -subj "/CN=skydive-agent" -config skydive-openssl.cnf
openssl x509 -req -days 365 -signkey agent/agent.domain.com.key -in agent/agent.domain.com.csr -out agent/agent.domain.com.crt -extfile skydive-openssl.cnf -extensions v3_req
```

### skydive-openssl.cnf
```
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
countryName = Country Name (2 letter code)
countryName_default = FR
stateOrProvinceName = State or Province Name (full name)
stateOrProvinceName_default = Paris
localityName = Locality Name (eg, city)
localityName_default = Paris
organizationalUnitName	= Organizational Unit Name (eg, section)
organizationalUnitName_default	= Skydive Team
commonName = skydive.domain.com
commonName_max	= 64

[ v3_req ]
# Extensions to add to a certificate request
basicConstraints = CA:TRUE
keyUsage = digitalSignature, keyEncipherment, keyCertSign
extendedKeyUsage = serverAuth,clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = agent.domain.com
DNS.2 = analyzer.domain.com
DNS.3 = localhost
IP.1 = 192.168.1.1
IP.2 = 192.168.69.14
IP.3 = 127.0.0.1
```
