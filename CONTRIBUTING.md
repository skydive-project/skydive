Contributing to Skydive
========================

Your contributions are more than welcome. Please read the following notes to
know about the process.

Making Changes:
---------------

Before pushing your changes, please make sure the tests and the linters pass:

* make fmt
* make test
* make functional

_Please note, make functional will create local network resources
(bridges, namespaces, ...)_

Once ready push your changes to your Skydive fork our CI will check your
pull request.

We're much more likely to approve your changes if you:

* Add tests for new functionality.
* Write a good commit message, please see how to write below.
* Maintain backward compatibility.

How To Submit Code
------------------

We use github.com's Pull Request feature to receive code contributions from
external contributors. See
https://help.github.com/articles/creating-a-pull-request/ for details on
how to create a request.

Commit message format
---------------------

The subject line of your commit should be in the following format:

area: summary

area :
Indicates the area of the Skydive to which the change applies :

* ui
* flow
* capture
* api
* graph
* cmd
* netlink
* ovsdb
* etc.

Feature request or bug report:
------------------------------

Be sure to search for existing bugs before you create another one.
Remember that contributions are always welcome!

https://github.com/skydive-project/skydive/issues

Core contributors:
------------------

* Sylvain Baubeau (lebauce)
* Nicolas Planel (nplanel)
* Sylvain Afchain (safchain)
* Aidan Shribman (hunchback)
* Masco Kaliyamoorthy (masco)
* Jean-Philippe Braun (eonpatapon)

Contact
-------

* Weekly meeting
    * [General - Weekly Hangout meeting](https://meet.jit.si/skydive-project) - every Thursday at 10:30 - 11:30 AM CEST
    * [Minutes](https://docs.google.com/document/d/1eri4vyjmAwxiWs2Kp4HYdCUDWACF_HXZDrDL8WcPF-o/edit?ts=5d946ad5#heading=h.g8f8gdfq0un9)

* Slack
    * https://skydive-project.slack.com
