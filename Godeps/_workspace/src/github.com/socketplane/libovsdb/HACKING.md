HACKING
=======

## Getting Set Up

Assuming you already have a Go environment set up.

    go get github.com/socketplane/libovsdb
    cd $GOPATH/src/github.com/socketplane/libovsdb

You can use [`hub`](https://hub.github.com) to fork the repo

    hub fork

... or alternatively, fork socketplane/libovsdb on GitHub and add your fork as a remote

    git remote add <github-user> git@github.com:<github-user>/libovsdb

## Hacking

Pull a local branch before you start developing.
Convention for branches is
    - `bug/1234` for a branch that addresses a specific bug
    - `feature/awesome` for a branch that implements an awesome feature

If your work is a minor, you can call the branch whatever you like (within reason).

## Committing

Before you submit code, you must agree to the [Developer Certificate of Origin](http://developercertificate.org)

    Developer Certificate of Origin
    Version 1.1

    Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
    660 York Street, Suite 102,
    San Francisco, CA 94110 USA

    Everyone is permitted to copy and distribute verbatim copies of this
    license document, but changing it is not allowed.


    Developer's Certificate of Origin 1.1

    By making a contribution to this project, I certify that:

    (a) The contribution was created in whole or in part by me and I
        have the right to submit it under the open source license
        indicated in the file; or

    (b) The contribution is based upon previous work that, to the best
        of my knowledge, is covered under an appropriate open source
        license and I have the right under that license to submit that
        work with modifications, whether created in whole or in part
        by me, under the same open source license (unless I am
        permitted to submit under a different license), as indicated
        in the file; or

    (c) The contribution was provided directly to me by some other
        person who certified (a), (b) or (c) and I have not modified
        it.

    (d) I understand and agree that this project and the contribution
        are public and that a record of the contribution (including all
        personal information I submit with it, including my sign-off) is
        maintained indefinitely and may be redistributed consistent with
        this project or the open source license(s) involved.

To verify that you agree, you must sign-off your commits.

    git commit -s

This adds the following to the bottom of you commit message

    Signed-off-by: John Doe <john@doe.io>

The name and email address used in the sign off are taken from your `user.name` and `user.email` settings in `git`. You can change these globally or locally using `git config` or from your `~/.gitconfig` file

## Before Making a Pull Request

    # Run all the tests
    fig up -d
    make test-all

    # Make sure your code is pretty
    go fmt

## Make a Pull Request

    git push <github-user> <branch-name>
    hub pull-request

... or if you still aren't using `hub` (which you should be by now) you can head over to [GitHub](http://github.com) and create a PR using the web interface

## Code Review

Once your patch has been submitted it will be scrutinized by your peers.
To make changes in response to comments...

    # Assuming you are already on the branch you raise the PR
    git push <github-user> --force

This will update the pull request, retrigger CI etc...

## Summary

We hope you find this guide helpful and are looking forward to your pull requests!
