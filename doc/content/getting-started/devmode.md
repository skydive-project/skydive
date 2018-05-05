---
date: 2016-11-28T15:21:44+01:00
title: Development
---

A Vagrant box with all the required dependencies to compile Skydive and run its
testsuite is [available](https://app.vagrantup.com/skydive/boxes/skydive-dev).

To use it :
```console
git clone https://github.com/skydive-project/skydive.git
cd contrib/dev
vagrant up
vagrant ssh
```

The box is available for both `VirtualBox` and `libvirt`.
