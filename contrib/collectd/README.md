# Skydive Collectd plugin

This packages implements a Collectd plugin that will send Collectd metrics to Skydive Agent.
Currently the metrics will reported on the `Host` node under the sub key `Collectd`.

# Installation

Please first follow the [Skydive build](http://skydive.network/documentation/build) documentation
in order to get Skydive sources installed properly, then :

```
cd $GOPATH/contrib/collectd
make
```

This will generate a shared object that can be placed in the collectd plugin folder.

# Configuration

```
LoadPlugin skydive
<Plugin skydive>
    Address "127.0.0.1:8081"
    Username ""
    Password ""
</Plugin>
```

The address has to be the Skydive Agent `listen` address.

## Contributing

Your contributions are more than welcome. Please check
https://github.com/skydive-project/skydive/blob/master/CONTRIBUTING.md
to know about the process.

## Contact

* IRC: #skydive-project on irc.freenode.net
* Mailing list: https://www.redhat.com/mailman/listinfo/skydive-dev

## License

This software is licensed under the Apache License, Version 2.0 (the
"License"); you may not use this software except in compliance with the
License.
You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


