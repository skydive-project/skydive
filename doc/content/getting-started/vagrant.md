---
date: 2016-11-28T15:21:44+01:00
title: vagrant
---

## Vagrant deployment

You can use Vagrant to deploy a Skydive environment with one virtual machine
running both Skydive analyzer and Elasticsearch, and two virtual machines with the
Skydive agent. This `Vagrantfile`, hosted in `contrib/vagrant` of the Git
repository, makes use of the
[libvirt Vagrant provider](https://github.com/vagrant-libvirt/vagrant-libvirt)
and uses Fedora as the box image.

```console
cd contrib/vagrant
vagrant up
```
