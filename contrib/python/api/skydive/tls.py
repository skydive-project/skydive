#
# Copyright (C) 2020 Sylvain Baubeau
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy ofthe License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specificlanguage governing permissions and
# limitations under the License.
#

import ssl


def create_ssl_context(insecure=False, cafile="", certfile="", keyfile=""):
    context = ssl.create_default_context()

    if insecure:
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE

    if cafile:
        context.load_verify_locations(cafile)

    if certfile and keyfile:
        context.load_cert_chain(certfile=certfile, keyfile=keyfile)

    return context
