#!/usr/bin/python

import argparse
import subprocess
import yaml
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('--conf', type=str, default="",
                    dest='configfile', required=True,
                    help='Jenkins configuration file')
parser.add_argument('--whitelist', type=str, default="",
                    dest='whitelist', required=True,
                    help='List of users to put in the ghprb white list')
parser.add_argument('--jobs', type=str, default="",
                    dest='jobs', required=True,
                    help='Jobs definitions file')
args = parser.parse_args()

whitelist = open(args.whitelist).read().split()
jobs = open(args.jobs).read()
jobs = jobs.replace("white-list: []", "white-list: " + repr(whitelist))

tmpjobs = tempfile.NamedTemporaryFile(mode="wt")
tmpjobs.write(jobs)
tmpjobs.flush()

subprocess.call(["jenkins-jobs", "--conf", args.configfile, "update", tmpjobs.name])
