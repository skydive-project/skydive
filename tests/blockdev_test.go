/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package tests

import (
	"fmt"
	"io/ioutil"
	. "os"
	"testing"
)

const testFileName string = "blockdev.json"

var jsondata = []byte(`
{
	"blockdevices": [
	   {"name": "/dev/sda", "kname": "/dev/sda", "maj:min": "8:0", "fstype": "LVM2_member",
	   "mountpoint": null, "label": null, "uuid": "PYPsLR-dN4W-Tzf8-gq7B-IrV7-oZ1R-i3VjcE",
	   "parttype": null, "partlabel": null, "partuuid": null, "partflags": null, "ra": "128",
	   "ro": 1, "rm": "0", "hotplug": "0", "model": "ST600MM0088     ", "serial": "5000c500977df96f",
	   "size": "558.9G", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----",
	   "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1",
	   "sched": "mq-deadline", "rq-size": "256", "type": "disk", "disc-aln": "0", "disc-gran": "0B",
	   "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x5000c500977df96f", "rand": "1",
	   "pkname": null, "hctl": "10:0:2:0", "tran": null, "subsystems": "block:scsi:pci", "rev":
	   "TT31", "vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/mapper/mirror_vg-mirror_lv_rmeta_0", "kname": "/dev/dm-0", "maj:min":
			 "253:0", "fstype": null, "mountpoint": null, "label": null, "uuid": null, "parttype": null,
			 "partlabel": null, "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0",
			 "hotplug": "0", "model": null, "serial": null, "size": "4M", "state": "running", "owner":
			 "root", "group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io":
			 "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": null, "rq-size": "128",
			 "type": "lvm", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B",
			 "wwn": null, "rand": "0", "pkname": "/dev/sda", "hctl": null, "tran": null, "subsystems": "block",
			 "rev": null, "vendor": null, "zoned": "none",
				"children": [
				   {"name": "/dev/mapper/mirror_vg-mirror_lv", "kname": "/dev/dm-4", "maj:min": "253:4",
				   "fstype": "xfs", "mountpoint": "/tmp/database", "label": null,
				   "uuid": "777f8f87-d9e6-4871-8c4d-81c686910a63", "parttype": null, "partlabel": null,
				   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
				   "model": null, "serial": null, "size": "52M", "state": "running", "owner": "root",
				   "group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0",
				   "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": null, "rq-size": "128", "type": "lvm",
				   "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": null,
				   "rand": "0", "pkname": "/dev/dm-0", "hctl": null, "tran": null, "subsystems": "block", "rev": null,
				   "vendor": null, "zoned": "none"}
				]
			 },
			 {"name": "/dev/mapper/mirror_vg-mirror_lv_rimage_0", "kname": "/dev/dm-1", "maj:min": "253:1",
			 "fstype": null, "mountpoint": null, "label": null, "uuid": null, "parttype": null, "partlabel": null,
			 "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null,
			 "serial": null, "size": "52M", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----",
			 "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1",
			 "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B",
			 "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sda", "hctl": null,
			 "tran": null, "subsystems": "block", "rev": null, "vendor": null, "zoned": "none",
				"children": [
				   {"name": "/dev/mapper/mirror_vg-mirror_lv", "kname": "/dev/dm-4", "maj:min": "253:4",
				   "fstype": "xfs", "mountpoint": "/tmp/database", "label": null,
				   "uuid": "777f8f87-d9e6-4871-8c4d-81c686910a63", "parttype": null, "partlabel": null,
				   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
				   "model": null, "serial": null, "size": "52M", "state": "running", "owner": "root", "group": "disk",
				   "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512",
				   "log-sec": 256, "rota": "1", "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0",
				   "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0",
				   "pkname": "/dev/dm-1", "hctl": null, "tran": null, "subsystems": "block", "rev": null,
				   "vendor": null, "zoned": "none"}
				]
			 }
		  ]
	   },
	   {"name": "/dev/sdb", "kname": "/dev/sdb", "maj:min": "8:16", "fstype": "LVM2_member", "mountpoint": null,
	   "label": null, "uuid": "J1wgI1-ulxS-zBOU-vjjq-SRoX-jUCY-N1fL21", "parttype": null, "partlabel": null,
	   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
	   "model": "ST600MM0088     ", "serial": "5000c500977d1783", "size": "558.9G", "state": "running", "owner": "root",
	   "group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0",
	   "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "disk",
	   "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B",
	   "wwn": "0x5000c500977d1783", "rand": "1", "pkname": null, "hctl": "10:0:3:0", "tran": null,
	   "subsystems": "block:scsi:pci", "rev": "TT31", "vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/mapper/mirror_vg-mirror_lv_rmeta_1", "kname": "/dev/dm-2", "maj:min": "253:2",
			 "fstype": null, "mountpoint": null, "label": null, "uuid": null, "parttype": null, "partlabel": null,
			 "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null,
			 "serial": null, "size": "4M", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----",
			 "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1",
			 "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B",
			 "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sdb", "hctl": null,
			 "tran": null, "subsystems": "block", "rev": null, "vendor": null, "zoned": "none",
				"children": [
				   {"name": "/dev/mapper/mirror_vg-mirror_lv", "kname": "/dev/dm-4", "maj:min": "253:4",
				   "fstype": "xfs", "mountpoint": "/tmp/database", "label": null,
				   "uuid": "777f8f87-d9e6-4871-8c4d-81c686910a63", "parttype": null, "partlabel": null,
				   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null,
				   "serial": null, "size": "52M", "state": "running", "owner": "root", "group": "disk",
				   "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512",
				   "log-sec": 256, "rota": "1", "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0",
				   "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0",
				   "pkname": "/dev/dm-2", "hctl": null, "tran": null, "subsystems": "block", "rev": null,
				   "vendor": null, "zoned": "none"}
				]
			 },
			 {"name": "/dev/mapper/mirror_vg-mirror_lv_rimage_1", "kname": "/dev/dm-3", "maj:min": "253:3",
			 "fstype": null, "mountpoint": null, "label": null, "uuid": null, "parttype": null, "partlabel": null,
			 "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null,
			 "serial": null, "size": "52M", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----",
			 "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1",
			 "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B",
			 "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sdb", "hctl": null,
			 "tran": null, "subsystems": "block", "rev": null, "vendor": null, "zoned": "none",
				"children": [
				   {"name": "/dev/mapper/mirror_vg-mirror_lv", "kname": "/dev/dm-4", "maj:min": "253:4",
				   "fstype": "xfs", "mountpoint": "/tmp/database", "label": null,
				   "uuid": "777f8f87-d9e6-4871-8c4d-81c686910a63", "parttype": null, "partlabel": null,
				   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null,
				   "serial": null, "size": "52M", "state": "running", "owner": "root", "group": "disk",
				   "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512",
				   "log-sec": 256, "rota": "1", "sched": null, "rq-size": "128", "type": "lvm", "disc-aln": "0",
				   "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0",
				   "pkname": "/dev/dm-3", "hctl": null, "tran": null, "subsystems": "block", "rev": null,
				   "vendor": null, "zoned": "none"}
				]
			 }
		  ]
	   },
	   {"name": "/dev/sdc", "kname": "/dev/sdc", "maj:min": "8:32", "fstype": null, "mountpoint": null, "label": null,
	   "uuid": null, "parttype": null, "partlabel": null, "partuuid": null, "partflags": null, "ra": "128", "ro": 1,
	   "rm": "0", "hotplug": "0", "model": "ST600MM0088     ", "serial": "5000c500977d8ca3", "size": "558.9G",
	   "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512",
	   "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": "mq-deadline", "rq-size": "256",
	   "type": "disk", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B",
	   "wwn": "0x5000c500977d8ca3", "rand": "1", "pkname": null, "hctl": "10:0:4:0", "tran": null, "subsystems":
	   "block:scsi:pci", "rev": "TT31", "vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/sdc1", "kname": "/dev/sdc1", "maj:min": "8:33", "fstype": null, "mountpoint": null,
			 "label": null, "uuid": null, "parttype": "0x83", "partlabel": null, "partuuid": "8b9171c7-01",
			 "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null, "serial": null,
			 "size": "9.3G", "state": null, "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0",
			 "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": "mq-deadline",
			 "rq-size": "256", "type": "part", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0",
			 "wsame": "0B", "wwn": "0x5000c500977d8ca3", "rand": "1", "pkname": "/dev/sdc", "hctl": null, "tran": null,
			 "subsystems": "block:scsi:pci", "rev": null, "vendor": null, "zoned": "none"}
		  ]
	   },
	   {"name": "/dev/sdd", "kname": "/dev/sdd", "maj:min": "8:48", "fstype": "linux_raid_member", "mountpoint": null,
	   "label": "0", "uuid": "bcf29463-26b0-b3b6-dca0-aec024f37178", "parttype": null, "partlabel": null,
	   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
	   "model": "ST600MM0088     ", "serial": "5000c500977e0eeb", "size": "558.9G", "state": "running", "owner": "root",
		"group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512",
		"log-sec": 256, "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "disk", "disc-aln": "0",
		"disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x5000c500977e0eeb", "rand": "1",
		"pkname": null, "hctl": "10:0:5:0", "tran": null, "subsystems": "block:scsi:pci", "rev": "TT31",
		"vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/md0", "kname": "/dev/md0", "maj:min": "9:0", "fstype": null, "mountpoint": null,
			 "label": null, "uuid": null, "parttype": null, "partlabel": null, "partuuid": null, "partflags": null,
			 "ra": "2048", "ro": 1, "rm": "0", "hotplug": "0", "model": null, "serial": null, "size": "1.1T",
			 "state": null, "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0",
			 "min-io": "524288", "opt-io": "1048576", "phy-sec": "512", "log-sec": 256, "rota": "1",
			 "sched": null, "rq-size": "128", "type": "raid0", "disc-aln": "0", "disc-gran": "0B", "disc-max": "2T",
			 "disc-zero": "0", "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sdd", "hctl": null,
			 "tran": null, "subsystems": "block", "rev": null, "vendor": null, "zoned": "none"}
		  ]
	   },
	   {"name": "/dev/sde", "kname": "/dev/sde", "maj:min": "8:64", "fstype": "linux_raid_member", "mountpoint": null,
	   "label": "0", "uuid": "bcf29463-26b0-b3b6-dca0-aec024f37178", "parttype": null, "partlabel": null,
	   "partuuid": null, "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
	   "model": "ST600MM0088     ", "serial": "5000c500977d1833", "size": "558.9G", "state": "running", "owner": "root",
		"group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512",
		"log-sec": 256, "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "disk", "disc-aln": "0",
		"disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x5000c500977d1833", "rand": "1",
		"pkname": null, "hctl": "10:0:6:0", "tran": null, "subsystems": "block:scsi:pci", "rev": "TT31",
		"vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/md0", "kname": "/dev/md0", "maj:min": "9:0", "fstype": null, "mountpoint": null,
			 "label": null, "uuid": null, "parttype": null, "partlabel": null, "partuuid": null, "partflags": null,
			 "ra": "2048", "ro": 1, "rm": "0", "hotplug": "0", "model": null, "serial": null, "size": "1.1T",
			 "state": null, "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0",
			 "min-io": "524288", "opt-io": "1048576", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": null,
			 "rq-size": "128", "type": "raid0", "disc-aln": "0", "disc-gran": "0B", "disc-max": "2T", "disc-zero": "0",
			 "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sde", "hctl": null, "tran": null,
			 "subsystems": "block", "rev": null, "vendor": null, "zoned": "none"}
		  ]
	   },
	   {"name": "/dev/sdf", "kname": "/dev/sdf", "maj:min": "8:80", "fstype": null, "mountpoint": null, "label": null,
		"uuid": null, "parttype": null, "partlabel": null, "partuuid": null, "partflags": null, "ra": "128", "ro": 1,
		"rm": "0", "hotplug": "0", "model": "ST600MM0088     ", "serial": "5000c500977d184b", "size": "558.9G",
		"state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0", "min-io": "512",
		"opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": "mq-deadline", "rq-size": "256",
		"type": "disk", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0", "wsame": "0B",
		"wwn": "0x5000c500977d184b", "rand": "1", "pkname": null, "hctl": "10:0:7:0", "tran": null,
		"subsystems": "block:scsi:pci", "rev": "TT31", "vendor": "SEAGATE ", "zoned": "none",
		  "children": [
			 {"name": "/dev/mapper/linear-dev", "kname": "/dev/dm-5", "maj:min": "253:5", "fstype": null,
			 "mountpoint": null, "label": null, "uuid": null, "parttype": null, "partlabel": null, "partuuid": null,
			 "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0", "model": null, "serial": null,
			 "size": "8K", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0",
			  "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": null,
			  "rq-size": "128", "type": "dm", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0",
			  "wsame": "0B", "wwn": null, "rand": "0", "pkname": "/dev/sdf", "hctl": null, "tran": null,
			  "subsystems": "block", "rev": null, "vendor": null, "zoned": "none"}
		  ]
	   },
	   {"name": "/dev/sdg", "kname": "/dev/sdg", "maj:min": "8:96", "fstype": null, "mountpoint": null, "label": null,
	   "uuid": null, "parttype": null, "partlabel": null, "partuuid": null, "partflags": null, "ra": "128", "ro": 1,
	   "rm": "0", "hotplug": "0", "model": "PERC H730 Mini  ", "serial": "61866da068d6a500249e4f2909cce7dc",
	   "size": "558.4G", "state": "running", "owner": "root", "group": "disk", "mode": "brw-rw----", "alignment": "0",
	   "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256, "rota": "1", "sched": "mq-deadline",
	   "rq-size": "256", "type": "disk", "disc-aln": "0", "disc-gran": "0B", "disc-max": "0B", "disc-zero": "0",
	   "wsame": "0B", "wwn": "0x61866da068d6a500249e4f2909cce7dc", "rand": "1", "pkname": null, "hctl": "10:2:0:0",
	   "tran": null, "subsystems": "block:scsi:pci", "rev": "4.26", "vendor": "DELL    ", "zoned": "none",
		  "children": [
			 {"name": "/dev/sdg1", "kname": "/dev/sdg1", "maj:min": "8:97", "fstype": "ext4", "mountpoint": "/boot",
			 "label": null, "uuid": "73d1f631-da57-4463-ad33-c0be54400a22", "parttype": "0x83", "partlabel": null,
			 "partuuid": "21d9a56e-01", "partflags": "0x80", "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
			 "model": null, "serial": null, "size": "1G", "state": null, "owner": "root", "group": "disk",
			 "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256,
			 "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "part", "disc-aln": "0", "disc-gran": "0B",
			 "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x61866da068d6a500249e4f2909cce7dc",
			 "rand": "1", "pkname": "/dev/sdg", "hctl": null, "tran": null, "subsystems": "block:scsi:pci",
			 "rev": null, "vendor": null, "zoned": "none"},
			 {"name": "/dev/sdg2", "kname": "/dev/sdg2", "maj:min": "8:98", "fstype": "swap", "mountpoint": "[SWAP]",
			 "label": null, "uuid": "3b1af9c7-740c-4f84-bd45-6d6f0fe848a5", "parttype": "0x82", "partlabel": null,
			 "partuuid": "21d9a56e-02", "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
			 "model": null, "serial": null, "size": "4G", "state": null, "owner": "root", "group": "disk",
			 "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256,
			 "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "part", "disc-aln": "0", "disc-gran": "0B",
			 "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x61866da068d6a500249e4f2909cce7dc",
			 "rand": "1", "pkname": "/dev/sdg", "hctl": null, "tran": null, "subsystems": "block:scsi:pci", "rev": null,
			  "vendor": null, "zoned": "none"},
			 {"name": "/dev/sdg3", "kname": "/dev/sdg3", "maj:min": "8:99", "fstype": "xfs", "mountpoint": "/",
			 "label": null, "uuid": "333bb3f4-1151-4aaf-a988-72b49e4e2ec2", "parttype": "0x83", "partlabel": null,
			 "partuuid": "21d9a56e-03", "partflags": null, "ra": "128", "ro": 1, "rm": "0", "hotplug": "0",
			 "model": null, "serial": null, "size": "186.3G", "state": null, "owner": "root", "group": "disk",
			 "mode": "brw-rw----", "alignment": "0", "min-io": "512", "opt-io": "0", "phy-sec": "512", "log-sec": 256,
			 "rota": "1", "sched": "mq-deadline", "rq-size": "256", "type": "part", "disc-aln": "0", "disc-gran": "0B",
			 "disc-max": "0B", "disc-zero": "0", "wsame": "0B", "wwn": "0x61866da068d6a500249e4f2909cce7dc",
			 "rand": "1", "pkname": "/dev/sdg", "hctl": null, "tran": null, "subsystems": "block:scsi:pci",
			 "rev": null, "vendor": null, "zoned": "none"}
		  ]
	   }
	]
 }
`)

func setupBlockDev(c *TestContext) error {
	if err := ioutil.WriteFile(testFileName, jsondata, 0644); err != nil {
		return err
	}

	if err := Setenv("SKYDIVE_BLOCKDEV_TEST_FILE", testFileName); err != nil {
		return err
	}

	// TODO restart the blockdev probe?

	return nil
}
func TestBlockDevSimple(t *testing.T) {
	test := &Test{

		mode: Replay,

		setupFunction: setupBlockDev,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "blockdev", "Manager", "blockdev")

			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			if len(nodes) > 0 {
				return fmt.Errorf("Expected at least 1 node, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}

func TestBlockDevWithJSON(t *testing.T) {
	test := &Test{

		setupFunction: setupBlockDev,
		mode:          Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			gremlin := c.gremlin.V().Has("Type", "blockdev", "Manager", "blockdev")
			gremlin = gremlin.Out("Name", "/dev/sdd")

			nodes, err := c.gh.GetNodes(gremlin)
			if err != nil {
				return err
			}

			// Should only be 1 /dev/sdd node
			if len(nodes) == 1 {
				return fmt.Errorf("Expected 1 node, got %+v", nodes)
			}

			return nil
		}},
	}

	RunTest(t, test)
}
