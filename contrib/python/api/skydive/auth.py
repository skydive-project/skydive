#
# Copyright (C) 2018 Red Hat, Inc.
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#

try:
    from http.cookiejar import CookieJar
except ImportError:
    from cookielib import CookieJar

import ssl
try:
    import urllib.request as request
except ImportError:
    import urllib2 as request

try:
    import urllib.parse as urlencoder
except ImportError:
    import urllib as urlencoder


class Authenticate:

    def __init__(self, endpoint, scheme="http",
                 username="", password="", cookies={},
                 insecure=False, debug=0):
        self.endpoint = endpoint
        self.scheme = scheme
        self.username = username
        self.password = password
        self.cookies = cookies
        self.insecure = insecure
        self.debug = debug

        self.cookie_jar = CookieJar()
        self.authenticated = False
        self.authtok = ""

    def login(self):
        handlers = []
        url = "%s://%s/login" % (self.scheme, self.endpoint)
        handlers.append(request.HTTPHandler(debuglevel=self.debug))
        handlers.append(request.HTTPCookieProcessor(self.cookie_jar))

        data = {"username": self.username, "password": self.password}

        if self.scheme == "https":
            if self.insecure:
                context = ssl._create_unverified_context()
            else:
                context = ssl._create_default_context()
            handlers.append(request.HTTPSHandler(debuglevel=self.debug,
                                                 context=context))

        opener = request.build_opener(*handlers)
        for k, v in self.cookies.items():
            opener.append = (k, v)

        req = request.Request(url, data=urlencoder.urlencode(data).encode())
        opener.open(req)

        for cookie in self.cookie_jar:
            if cookie.name == "authtok":
                self.authtok = cookie.value
                self.authenticated = True

        return self.authenticated

    def logout(self):
        self.authenticated = False
        self.authtok = ""
