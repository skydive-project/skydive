---
date: 2016-05-06T11:02:01+02:00
title: Contributing
---

This project accepts contributions. Skydive uses the Gerrit workflow
through Software Factory.

http://softwarefactory-project.io/r/#/q/project:skydive

## Setting up your environment

```console
git clone https://softwarefactory-project.io/r/skydive
```

git-review installation :

```console
yum install git-review
```

or


```console
apt-get install git-review
```

or to get the latest version

```console
sudo pip install git-review
```

## Starting a Change

Create a topic branch :

```console
git checkout -b TOPIC-BRANCH
```

Submit your change :

```console
git review
```

Updating your Change :

```console
git commit -a --amend
git review
```

For a more complete documentation about
[how to contribute to a gerrit hosted project](https://gerrit-documentation.storage.googleapis.com/Documentation/2.12/intro-quick.html#_the_life_and_times_of_a_change).
