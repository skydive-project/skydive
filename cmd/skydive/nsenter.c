// Copyright (c) 2019 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#define _GNU_SOURCE 
#include <sched.h>

#include <linux/limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <fcntl.h>
#include <getopt.h>

static struct namespace {
	const char *name;
	const char *path;
	int nstype;
	int fd;
} namespaces[] = {
	{ .name = "user", .nstype = CLONE_NEWUSER, .fd = -1, .path = NULL },
	{ .name = "ipc", .nstype = CLONE_NEWIPC, .fd = -1, .path = NULL },
	{ .name = "pid", .nstype = CLONE_NEWPID, .fd = -1, .path = NULL },
	{ .name = "net", .nstype = CLONE_NEWNET, .fd = -1 , .path = NULL },
	{ .name = "mnt", .nstype = CLONE_NEWNS, .fd = -1, .path = NULL },
	{ .name = NULL, .nstype = 0, .fd = -1 }
};

static const struct option longopts[] = {
	{ "mount", required_argument, NULL, 'm' },
	{ "ipc", required_argument, NULL, 'i' },
	{ "pid", required_argument, NULL, 'p' },
	{ "net", required_argument, NULL, 'n' },
	{ "user", required_argument, NULL, 'u' },
	{ "verbose", no_argument, NULL, 'v' },
	{ NULL, 0, NULL, 0 },
};

void set_namespace_path(int nstype, const char *path) {
	struct namespace *ns;

	for (ns = namespaces; ns->nstype; ns++) {
		if (ns->nstype == nstype) {
			printf("set namespace path %s %s\n", ns->name, path);
			ns->path = strdup(path);
			return;
		}
	}
}

void open_namespaces() {
	struct namespace *ns;

	for (ns = namespaces; ns->nstype; ns++) {
		if (ns->path != NULL) {
			printf("opening namespace %s %s\n", ns->path, ns->name);
			ns->fd = open(ns->path, O_RDONLY);
			if (ns->fd < 0) {
				perror("Failed to open namespace");
				exit(1);
			}
		}
	}
}

void set_namespaces() {
	struct namespace *ns;
	for (ns = namespaces; ns->nstype; ns++) {
		if (ns->fd > 0) {
			if (setns(ns->fd, ns->nstype)) {
				perror("Failed to switch to namespace");
				exit(1);
			}
			fprintf(stderr, "Switched to %s\n", ns->name);
			close(ns->fd);
			ns->fd = -1;
		}
	}
}

void nsexec(void)
{
	FILE *f = fopen("/proc/self/cmdline", "r");
	if (f == NULL) {
		perror("failed to open file");
		exit(1);
	}

	int argc = 0;
	char *arg = NULL;
	size_t size;
	char *argv[PATH_MAX];
	for (; argc < PATH_MAX && (getdelim(&arg, &size, 0, f) != -1); argc++) {
		argv[argc] = strdup(arg);
	}
	free(arg);
	fclose(f);
	argv[argc] = NULL;

	int verbose = 0;
	int need_to_fork = 0;
	int option_index;
	char c;
	while ((c = getopt_long(argc, argv, ":m:p:u:i:n:v:",
			longopts, &option_index)) != -1) {
		switch (c) {
		case 'u':
			set_namespace_path(CLONE_NEWUSER, optarg);
			break;
		case 'p':
			set_namespace_path(CLONE_NEWPID, optarg);
			need_to_fork = 1;
			break;
		case 'n':
			set_namespace_path(CLONE_NEWNET, optarg);
			break;
		case 'm':
			set_namespace_path(CLONE_NEWNS, optarg);
			break;
		case 'i':
			set_namespace_path(CLONE_NEWIPC, optarg);
			break;
		case 'v':
			verbose = 1;
			break;
		}
	}

	if (optind < argc) {
		if (strcmp(argv[optind], "seed")) {
			return;
		} 
	}

	if (verbose) fprintf(stderr, "Opening namespaces\n");
	open_namespaces();

	if (verbose) fprintf(stderr, "Jumping into namespaces\n");
	set_namespaces();

	if (need_to_fork) {
		if (verbose) fprintf(stderr, "Forking");
		int pid = fork();
		if (pid < 0) {
			perror("failed to fork");
			return;
		}

		if (pid > 0) {
			int wstatus;
			waitpid(pid, &wstatus, 0);
			exit(WEXITSTATUS(wstatus));
		}
	}
}