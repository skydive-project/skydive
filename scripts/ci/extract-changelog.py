#!/usr/bin/python

import sys

if len(sys.argv) < 2:
    print("Usage: %s <changelog> [<version>]" % (sys.argv[0],))
    sys.exit(1)

changelog = open(sys.argv[1]).readlines()
version = "latest"
if len(sys.argv) > 2:
    version = sys.argv[2]

start = 0
end = -1

for i, line in enumerate(changelog):
    if line.startswith("## "):
        if start == 0 and (version == "latest" or line.startswith("## [%s]" % (version,))):
            start = i
        elif start != 0:
            end = i
            break

if start != 0:
    print ''.join(changelog[start+1:end]).strip()
