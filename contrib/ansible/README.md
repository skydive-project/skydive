# Skydive Ansible

This repository contains [Ansible](https://www.ansible.com/) roles and
playbooks to install Skydive analyzers and agents.

## Requirements

- Ansible >= 2.4.3.0
- Jinja >= 2.7
- Passlib >= 1.6

## all-in-one localhost Installation

sudo ansible-playbook -i inventory/hosts.localhost playbook.yml.sample

## Deployment mode

There are two main deployment modes available :

* binary (default)
* container

The binary mode download the latest stable version of Skydive.
The container mode uses the latest Docker container built. Docker
has to be deployed on the hosts to use this mode.

In order to select the mode, the `skydive_deployment_mode` has to be
set.

## Roles

Basically there are only two roles :

- skydive_analyzer
- skydive_agent

## Variables available

| Variable                    | Description                                   |
| --------------------------- | --------------------------------------------- |
| skydive_topology_probes     | Defines topology probes used by the agents    |
| skydive_fabric              | Fabric definition                             |
| skydive_etcd_embedded       | Use embedded Etcd (yes/no)                    |
| skydive_etcd_port           | Defines Etcd port                             |
| skydive_etcd_servers        | Defines Etcd servers if not embedded          |
| skydive_analyzer_port       | Defines analyzer listen port                  |
| skydive_analyzer_ip         | Defines anakyzer listen IP                    |
| skydive_deployment_mode     | Specify the deployment mode                   |
| skydive_auth_type           | Specify the authentication type               |
| skydive_basic_auth_file     | Secret file for basic authentication          |
| skydive_username            | Username used for the basic authentication    |
| skydive_password            | Password used for the basic authentication    |
| skydive_config_file         | Specify the configuration path                |
| skydive_flow_protocol       | Specify the flow protocol used                |
| skydive_extra_config        | Defines any extra config parameter            |
| skydive_nic                 | Specify the listen interface                  |
| os_auth_url                 | OpenStack authentication url                  |
| os_username                 | OpenStack username                            |
| os_password                 | OpenStack password                            |
| os_tenant_name              | OpenStack tenant name                         |
| os_domain_name              | OpenStack domain name                         |
| os_endpoint_type            | OpenStack endpoint type                       |

## How to configure Skydive

Every configuration parameter of the Skydive configuration file can be
overridden through an unique Ansible variable : `skydive_extra_config`.

To activate both `docker` and `socketinfo` probe you can use :

```
skydive_extra_config={'agent.topology.probes': ['socketinfo', 'docker'], 'logging.level': 'DEBUG'}
```

## Examples

Some examples are present in the [inventory](inventory/) folder.

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
