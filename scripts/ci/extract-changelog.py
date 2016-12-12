#!/usr/bin/python

import sys

if len(sys.argv) < 3:
    print("Usage: %s <changelog> <version>" % (sys.argv[0],))
    sys.exit(1)

changelog = open(sys.argv[1]).readlines()
version = sys.argv[2]

start = 0
end = -1

for i, line in enumerate(changelog):
    if line.startswith("## "):
        if line.startswith("## [%s]" % (version,)) and start == 0:
            start = i
        elif start != 0:
            end = i
            break

if start != 0:
    print ''.join(changelog[start+1:end]).strip()
