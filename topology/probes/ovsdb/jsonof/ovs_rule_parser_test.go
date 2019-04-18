/*
 * Copyright (C) 2018 Orange.
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

package jsonof

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

var testScans = []struct {
	name     string
	openflow string
	tokens   []Token
}{
	{
		"default",
		" cookie=0x0, duration=337052.111s, table=0, n_packets=0, n_bytes=0, priority=0 actions=NORMAL",
		[]Token{
			tSpace, tText, tEqual, tText, tComma, tSpace, tText, tEqual, tText, tComma, tSpace,
			tText, tEqual, tText, tComma, tSpace, tText, tEqual, tText, tComma, tSpace,
			tText, tEqual, tText, tComma, tSpace, tText, tEqual, tText, tSpace,
			tText, tEqual, tText, tEOF,
		},
	},
	{
		"long rule",
		" cookie=0x20, duration=57227.249s, table=21, priority=1,dl_src=01:00:00:00:00:00/01:00:00:00:00:00 actions=drop",
		[]Token{
			tSpace, tText, tEqual, tText, tComma, tSpace,
			tText, tEqual, tText, tComma, tSpace,
			tText, tEqual, tText, tComma, tSpace,
			tText, tEqual, tText, tComma, tText, tEqual, tText,
			tSpace, tText, tEqual, tText, tEOF,
		},
	},
}

var testParses = []struct {
	name     string
	openflow string
	json     string
	pretty   string
}{
	{
		"controller action",
		" cookie=0x1, duration=1.11s, table=11, n_packets=1, n_bytes=1, priority=11,icmp1,metadata=0x1,ipv1_dst=fe11::11:1ff:fe11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_target=fe11::11:1ff:fe11:11 actions=controller(userdata=11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11)",
		`{"Cookie":1,"Table":11,"Priority":11,"Meta":[{"Key":"duration","Value":"1.11s"},{"Key":"n_packets","Value":"1"},{"Key":"n_bytes","Value":"1"}],"Filters":[{"Key":"icmp1","Value":""},{"Key":"metadata","Value":"0x1"},{"Key":"ipv1_dst","Value":"fe11::11:1ff:fe11:11"},{"Key":"nw_ttl","Value":"11"},{"Key":"icmp_type","Value":"11"},{"Key":"icmp_code","Value":"1"},{"Key":"nd_target","Value":"fe11::11:1ff:fe11:11"}],"Actions":[{"Function":"controller","Arguments":[{"Function":"11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11","Key":"userdata"}]}]}`,
		"cookie=0x1, table=11, duration=1.11s, n_packets=1, n_bytes=1, priority=11,icmp1,metadata=0x1,ipv1_dst=fe11::11:1ff:fe11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_target=fe11::11:1ff:fe11:11 actions=controller(userdata=11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11.11)",
	},
	{
		"mac address field",
		" cookie=0x1, duration=1.11s, table=11, n_packets=1, n_bytes=1, priority=11,icmp1,reg11=0x1,metadata=0x1,dl_src=f1:11:11:11:11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_sll=11:11:11:11:11:11 actions=resubmit(,11)",
		`{"Cookie":1,"Table":11,"Priority":11,"Meta":[{"Key":"duration","Value":"1.11s"},{"Key":"n_packets","Value":"1"},{"Key":"n_bytes","Value":"1"}],"Filters":[{"Key":"icmp1","Value":""},{"Key":"reg11","Value":"0x1"},{"Key":"metadata","Value":"0x1"},{"Key":"dl_src","Value":"f1:11:11:11:11:11"},{"Key":"nw_ttl","Value":"11"},{"Key":"icmp_type","Value":"11"},{"Key":"icmp_code","Value":"1"},{"Key":"nd_sll","Value":"11:11:11:11:11:11"}],"Actions":[{"Function":"resubmit","Arguments":[null,{"Function":"11"}]}]}`,
		"cookie=0x1, table=11, duration=1.11s, n_packets=1, n_bytes=1, priority=11,icmp1,reg11=0x1,metadata=0x1,dl_src=f1:11:11:11:11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_sll=11:11:11:11:11:11 actions=resubmit(,11)",
	},
	{
		"ip v6 field",
		" cookie=0x1, duration=1.11s, table=11, n_packets=1, n_bytes=1, priority=11,icmp1,reg11=0x1,metadata=0x1,ipv1_dst=ff11::1:ff11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_target=fe11::11:1ff:fe11:11 actions=resubmit(,11)",
		`{"Cookie":1,"Table":11,"Priority":11,"Meta":[{"Key":"duration","Value":"1.11s"},{"Key":"n_packets","Value":"1"},{"Key":"n_bytes","Value":"1"}],"Filters":[{"Key":"icmp1","Value":""},{"Key":"reg11","Value":"0x1"},{"Key":"metadata","Value":"0x1"},{"Key":"ipv1_dst","Value":"ff11::1:ff11:11"},{"Key":"nw_ttl","Value":"11"},{"Key":"icmp_type","Value":"11"},{"Key":"icmp_code","Value":"1"},{"Key":"nd_target","Value":"fe11::11:1ff:fe11:11"}],"Actions":[{"Function":"resubmit","Arguments":[null,{"Function":"11"}]}]}`,
		"cookie=0x1, table=11, duration=1.11s, n_packets=1, n_bytes=1, priority=11,icmp1,reg11=0x1,metadata=0x1,ipv1_dst=ff11::1:ff11:11,nw_ttl=11,icmp_type=11,icmp_code=1,nd_target=fe11::11:1ff:fe11:11 actions=resubmit(,11)",
	},
	{
		"learn action",
		" cookie=0x0, duration=0.009s, table=0, n_packets=0, n_bytes=0, reset_counts actions=learn(table=1,hard_timeout=60,NXM_OF_VLAN_TCI[0..11],NXM_OF_ETH_DST[]=NXM_OF_ETH_SRC[],output:NXM_OF_IN_PORT[]),resubmit(,1)",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.009s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":null,"Actions":[{"Function":"learn","Arguments":[{"Function":"1","Key":"table"},{"Function":"60","Key":"hard_timeout"},{"Function":"range","Arguments":[{"Function":"NXM_OF_VLAN_TCI"},{"Function":"0"},{"Function":"11"}]},{"Function":"=","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_OF_ETH_DST"}]},{"Function":"range","Arguments":[{"Function":"NXM_OF_ETH_SRC"}]}]},{"Function":"output","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_OF_IN_PORT"}]}]}]},{"Function":"resubmit","Arguments":[null,{"Function":"1"}]}]}`,
		"cookie=0x0, table=0, duration=0.009s, n_packets=0, n_bytes=0, reset_counts, priority=0 actions=learn(table=1,hard_timeout=60,NXM_OF_VLAN_TCI[0..11],NXM_OF_ETH_DST[]:=NXM_OF_ETH_SRC[],output(NXM_OF_IN_PORT[])),resubmit(,1)",
	},
	{
		"move action",
		" cookie=0xd, duration=0.412s, table=0, n_packets=0, n_bytes=0, reset_counts dl_src=60:66:66:66:00:03 actions=pop_mpls:0x0800,move:NXM_OF_IP_DST[]->NXM_OF_IP_SRC[],CONTROLLER:65535",
		`{"Cookie":13,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.412s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"dl_src","Value":"60:66:66:66:00:03"}],"Actions":[{"Function":"pop_mpls","Arguments":[{"Function":"0x0800"}]},{"Function":"move","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_DST"}]},{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_SRC"}]}]},{"Function":"CONTROLLER","Arguments":[{"Function":"65535"}]}]}`,
		"cookie=0xd, table=0, duration=0.412s, n_packets=0, n_bytes=0, reset_counts, priority=0,dl_src=60:66:66:66:00:03 actions=pop_mpls(0x0800),move(NXM_OF_IP_DST[],NXM_OF_IP_SRC[]),CONTROLLER(65535)",
	},
	{
		"push/pop actions",
		" cookie=0xd, duration=0.412s, table=0, n_packets=0, n_bytes=0, reset_counts dl_src=60:66:66:66:00:04 actions=pop_mpls:0x0800,push:NXM_OF_IP_DST[],pop:NXM_OF_IP_SRC[],CONTROLLER:65535",
		`{"Cookie":13,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.412s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"dl_src","Value":"60:66:66:66:00:04"}],"Actions":[{"Function":"pop_mpls","Arguments":[{"Function":"0x0800"}]},{"Function":"push","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_DST"}]}]},{"Function":"pop","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_SRC"}]}]},{"Function":"CONTROLLER","Arguments":[{"Function":"65535"}]}]}`,
		"cookie=0xd, table=0, duration=0.412s, n_packets=0, n_bytes=0, reset_counts, priority=0,dl_src=60:66:66:66:00:04 actions=pop_mpls(0x0800),push(NXM_OF_IP_DST[]),pop(NXM_OF_IP_SRC[]),CONTROLLER(65535)",
	},
	{
		"set_field action",
		" cookie=0xa, duration=0.414s, table=0, n_packets=1, n_bytes=42, reset_counts dl_src=40:44:44:44:44:42 actions=push_mpls:0x8847,set_field:10/0xfffff->mpls_label,set_field:3/0x7->mpls_tc,set_field:10.0.0.1->ip_dst,CONTROLLER:65535",
		`{"Cookie":10,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.414s"},{"Key":"n_packets","Value":"1"},{"Key":"n_bytes","Value":"42"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"dl_src","Value":"40:44:44:44:44:42"}],"Actions":[{"Function":"push_mpls","Arguments":[{"Function":"0x8847"}]},{"Function":"set_field","Arguments":[{"Function":"10"},{"Function":"0xfffff"},{"Function":"mpls_label"}]},{"Function":"set_field","Arguments":[{"Function":"3"},{"Function":"0x7"},{"Function":"mpls_tc"}]},{"Function":"set_field","Arguments":[{"Function":"10.0.0.1"},null,{"Function":"ip_dst"}]},{"Function":"CONTROLLER","Arguments":[{"Function":"65535"}]}]}`,
		"cookie=0xa, table=0, duration=0.414s, n_packets=1, n_bytes=42, reset_counts, priority=0,dl_src=40:44:44:44:44:42 actions=push_mpls(0x8847),set_field(10,0xfffff,mpls_label),set_field(3,0x7,mpls_tc),set_field(10.0.0.1,,ip_dst),CONTROLLER(65535)",
	},
	{
		"bundle_load action",
		" cookie=0xd, duration=97.230s, table=0, n_packets=3, n_bytes=186, reset_counts dl_src=60:66:66:66:00:06 actions=pop_mpls:0x0800,bundle_load(eth_src,50,hrw,ofport,NXM_OF_IP_SRC[0..15],slaves:1,2),CONTROLLER:65535",
		`{"Cookie":13,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"97.230s"},{"Key":"n_packets","Value":"3"},{"Key":"n_bytes","Value":"186"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"dl_src","Value":"60:66:66:66:00:06"}],"Actions":[{"Function":"pop_mpls","Arguments":[{"Function":"0x0800"}]},{"Function":"bundle_load","Arguments":[{"Function":"eth_src"},{"Function":"50"},{"Function":"hrw"},{"Function":"ofport"},{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_SRC"},{"Function":"0"},{"Function":"15"}]},{"Function":"slaves","Arguments":[{"Function":"1"}]},{"Function":"2"}]},{"Function":"CONTROLLER","Arguments":[{"Function":"65535"}]}]}`,
		"cookie=0xd, table=0, duration=97.230s, n_packets=3, n_bytes=186, reset_counts, priority=0,dl_src=60:66:66:66:00:06 actions=pop_mpls(0x0800),bundle_load(eth_src,50,hrw,ofport,NXM_OF_IP_SRC[0..15],slaves(1),2),CONTROLLER(65535)",
	},
	{
		"load action",
		" cookie=0x0, duration=0.018s, table=0, n_packets=0, n_bytes=0, reset_counts priority=100,in_port=1 actions=load:0x1->NXM_NX_REG13[],load:0x2->NXM_NX_REG11[],load:0x3->NXM_NX_REG12[],load:0x1->OXM_OF_METADATA[],load:0x1->NXM_NX_REG14[],resubmit(,8)",
		`{"Cookie":0,"Table":0,"Priority":100,"Meta":[{"Key":"duration","Value":"0.018s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"in_port","Value":"1"}],"Actions":[{"Function":"load","Arguments":[{"Function":"0x1"},{"Function":"range","Arguments":[{"Function":"NXM_NX_REG13"}]}]},{"Function":"load","Arguments":[{"Function":"0x2"},{"Function":"range","Arguments":[{"Function":"NXM_NX_REG11"}]}]},{"Function":"load","Arguments":[{"Function":"0x3"},{"Function":"range","Arguments":[{"Function":"NXM_NX_REG12"}]}]},{"Function":"load","Arguments":[{"Function":"0x1"},{"Function":"range","Arguments":[{"Function":"OXM_OF_METADATA"}]}]},{"Function":"load","Arguments":[{"Function":"0x1"},{"Function":"range","Arguments":[{"Function":"NXM_NX_REG14"}]}]},{"Function":"resubmit","Arguments":[null,{"Function":"8"}]}]}`,
		"cookie=0x0, table=0, duration=0.018s, n_packets=0, n_bytes=0, reset_counts, priority=100,in_port=1 actions=load(0x1,NXM_NX_REG13[]),load(0x2,NXM_NX_REG11[]),load(0x3,NXM_NX_REG12[]),load(0x1,OXM_OF_METADATA[]),load(0x1,NXM_NX_REG14[]),resubmit(,8)",
	},
	{
		"encap",
		" cookie=0x0, duration=0.007s, table=0, n_packets=0, n_bytes=0, ip,in_port=1 actions=encap(nsh(md_type=2,tlv(0x1000,10,0x12345678),tlv(0x2000,20,0xfedcba9876543210))),set_field:0x1234->nsh_spi,encap(ethernet),set_field:11:22:33:44:55:66->eth_dst,output:3",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.007s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"}],"Filters":[{"Key":"ip","Value":""},{"Key":"in_port","Value":"1"}],"Actions":[{"Function":"encap","Arguments":[{"Function":"nsh","Arguments":[{"Function":"2","Key":"md_type"},{"Function":"tlv","Arguments":[{"Function":"0x1000"},{"Function":"10"},{"Function":"0x12345678"}]},{"Function":"tlv","Arguments":[{"Function":"0x2000"},{"Function":"20"},{"Function":"0xfedcba9876543210"}]}]}]},{"Function":"set_field","Arguments":[{"Function":"0x1234"},null,{"Function":"nsh_spi"}]},{"Function":"encap","Arguments":[{"Function":"ethernet"}]},{"Function":"set_field","Arguments":[{"Function":"11:22:33:44:55:66"},null,{"Function":"eth_dst"}]},{"Function":"output","Arguments":[{"Function":"3"}]}]}`,
		"cookie=0x0, table=0, duration=0.007s, n_packets=0, n_bytes=0, priority=0,ip,in_port=1 actions=encap(nsh(md_type=2,tlv(0x1000,10,0x12345678),tlv(0x2000,20,0xfedcba9876543210))),set_field(0x1234,,nsh_spi),encap(ethernet),set_field(11:22:33:44:55:66,,eth_dst),output(3)",
	},
	{
		"ct action",
		" cookie=0x0, duration=0.007s, table=5, n_packets=0, n_bytes=0, reset_counts priority=10,ct_state=+new-rel,ip,reg2=0x1 actions=ct(commit,zone=NXM_NX_REG4[0..15],exec(move:NXM_NX_REG3[]->NXM_NX_CT_MARK[],move:NXM_NX_REG1[]->NXM_NX_CT_LABEL[96..127])),resubmit(,6)",
		`{"Cookie":0,"Table":5,"Priority":10,"Meta":[{"Key":"duration","Value":"0.007s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"ct_state","Value":"+new-rel"},{"Key":"ip","Value":""},{"Key":"reg2","Value":"0x1"}],"Actions":[{"Function":"ct","Arguments":[{"Function":"commit"},{"Function":"range","Arguments":[{"Function":"NXM_NX_REG4"},{"Function":"0"},{"Function":"15"}],"Key":"zone"},{"Function":"exec","Arguments":[{"Function":"move","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_NX_REG3"}]},{"Function":"range","Arguments":[{"Function":"NXM_NX_CT_MARK"}]}]},{"Function":"move","Arguments":[{"Function":"range","Arguments":[{"Function":"NXM_NX_REG1"}]},{"Function":"range","Arguments":[{"Function":"NXM_NX_CT_LABEL"},{"Function":"96"},{"Function":"127"}]}]}]}]},{"Function":"resubmit","Arguments":[null,{"Function":"6"}]}]}`,
		"cookie=0x0, table=5, duration=0.007s, n_packets=0, n_bytes=0, reset_counts, priority=10,ct_state=+new-rel,ip,reg2=0x1 actions=ct(commit,zone=NXM_NX_REG4[0..15],exec(move(NXM_NX_REG3[],NXM_NX_CT_MARK[]),move(NXM_NX_REG1[],NXM_NX_CT_LABEL[96..127]))),resubmit(,6)",
	},
	{
		"sample action",
		" cookie=0x0, duration=0.007s, table=0, n_packets=0, n_bytes=0, reset_counts in_port=3 actions=sample(probability=65535,collector_set_id=1,obs_domain_id=0,obs_point_id=0,sampling_port=1),output:1,sample(probability=65535,collector_set_id=1,obs_domain_id=0,obs_point_id=0,sampling_port=2),output:2",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.007s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"in_port","Value":"3"}],"Actions":[{"Function":"sample","Arguments":[{"Function":"65535","Key":"probability"},{"Function":"1","Key":"collector_set_id"},{"Function":"0","Key":"obs_domain_id"},{"Function":"0","Key":"obs_point_id"},{"Function":"1","Key":"sampling_port"}]},{"Function":"output","Arguments":[{"Function":"1"}]},{"Function":"sample","Arguments":[{"Function":"65535","Key":"probability"},{"Function":"1","Key":"collector_set_id"},{"Function":"0","Key":"obs_domain_id"},{"Function":"0","Key":"obs_point_id"},{"Function":"2","Key":"sampling_port"}]},{"Function":"output","Arguments":[{"Function":"2"}]}]}`,
		"cookie=0x0, table=0, duration=0.007s, n_packets=0, n_bytes=0, reset_counts, priority=0,in_port=3 actions=sample(probability=65535,collector_set_id=1,obs_domain_id=0,obs_point_id=0,sampling_port=1),output(1),sample(probability=65535,collector_set_id=1,obs_domain_id=0,obs_point_id=0,sampling_port=2),output(2)",
	},
	{
		"clone action",
		" cookie=0x0, duration=0.006s, table=0, n_packets=0, n_bytes=0, reset_counts ip,in_port=1 actions=clone(set_field:192.168.3.3->ip_src),clone(set_field:192.168.4.4->ip_dst,output:2),clone(set_field:80:81:81:81:81:81->eth_src,set_field:192.168.5.5->ip_dst,output:3),output:4",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.006s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"ip","Value":""},{"Key":"in_port","Value":"1"}],"Actions":[{"Function":"clone","Arguments":[{"Function":"set_field","Arguments":[{"Function":"192.168.3.3"},null,{"Function":"ip_src"}]}]},{"Function":"clone","Arguments":[{"Function":"set_field","Arguments":[{"Function":"192.168.4.4"},null,{"Function":"ip_dst"}]},{"Function":"output","Arguments":[{"Function":"2"}]}]},{"Function":"clone","Arguments":[{"Function":"set_field","Arguments":[{"Function":"80:81:81:81:81:81"},null,{"Function":"eth_src"}]},{"Function":"set_field","Arguments":[{"Function":"192.168.5.5"},null,{"Function":"ip_dst"}]},{"Function":"output","Arguments":[{"Function":"3"}]}]},{"Function":"output","Arguments":[{"Function":"4"}]}]}`,
		"cookie=0x0, table=0, duration=0.006s, n_packets=0, n_bytes=0, reset_counts, priority=0,ip,in_port=1 actions=clone(set_field(192.168.3.3,,ip_src)),clone(set_field(192.168.4.4,,ip_dst),output(2)),clone(set_field(80:81:81:81:81:81,,eth_src),set_field(192.168.5.5,,ip_dst),output(3)),output(4)",
	},
	{
		"ct_state field",
		" cookie=0x0, duration=0.030s, table=1, n_packets=0, n_bytes=0, reset_counts ct_state=-rel+rpl-inv+trk,ip,reg3=0x2 actions=set_field:0x1->reg0,resubmit(,3,ct),resubmit(,4)",
		`{"Cookie":0,"Table":1,"Priority":0,"Meta":[{"Key":"duration","Value":"0.030s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"ct_state","Value":"-rel+rpl-inv+trk"},{"Key":"ip","Value":""},{"Key":"reg3","Value":"0x2"}],"Actions":[{"Function":"set_field","Arguments":[{"Function":"0x1"},null,{"Function":"reg0"}]},{"Function":"resubmit","Arguments":[null,{"Function":"3"},{"Function":"ct"}]},{"Function":"resubmit","Arguments":[null,{"Function":"4"}]}]}`,
		"cookie=0x0, table=1, duration=0.030s, n_packets=0, n_bytes=0, reset_counts, priority=0,ct_state=-rel+rpl-inv+trk,ip,reg3=0x2 actions=set_field(0x1,,reg0),resubmit(,3,ct),resubmit(,4)",
	},
	{
		"write_actions",
		" cookie=0x0, duration=0.009s, table=0, n_packets=0, n_bytes=0, reset_counts in_port=2 actions=group:1,write_actions(group:2,group:3,output:6)",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.009s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"in_port","Value":"2"}],"Actions":[{"Function":"group","Arguments":[{"Function":"1"}]},{"Function":"write_actions","Arguments":[{"Function":"group","Arguments":[{"Function":"2"}]},{"Function":"group","Arguments":[{"Function":"3"}]},{"Function":"output","Arguments":[{"Function":"6"}]}]}]}`,
		"cookie=0x0, table=0, duration=0.009s, n_packets=0, n_bytes=0, reset_counts, priority=0,in_port=2 actions=group(1),write_actions(group(2),group(3),output(6))",
	},
	{
		"multipath action",
		" cookie=0xd, duration=0.009s, table=0, n_packets=0, n_bytes=0, reset_counts dl_src=60:66:66:66:00:05 actions=pop_mpls:0x0800,multipath(eth_src,50,modulo_n,1,0,NXM_OF_IP_SRC[0..7]),CONTROLLER:65535",
		`{"Cookie":13,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.009s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"dl_src","Value":"60:66:66:66:00:05"}],"Actions":[{"Function":"pop_mpls","Arguments":[{"Function":"0x0800"}]},{"Function":"multipath","Arguments":[{"Function":"eth_src"},{"Function":"50"},{"Function":"modulo_n"},{"Function":"1"},{"Function":"0"},{"Function":"range","Arguments":[{"Function":"NXM_OF_IP_SRC"},{"Function":"0"},{"Function":"7"}]}]},{"Function":"CONTROLLER","Arguments":[{"Function":"65535"}]}]}`,
		"cookie=0xd, table=0, duration=0.009s, n_packets=0, n_bytes=0, reset_counts, priority=0,dl_src=60:66:66:66:00:05 actions=pop_mpls(0x0800),multipath(eth_src,50,modulo_n,1,0,NXM_OF_IP_SRC[0..7]),CONTROLLER(65535)",
	},
	{
		"enqueue action",
		" cookie=0x0, duration=9.471s, table=0, n_packets=0, n_bytes=0, actions=enqueue:123:456",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"9.471s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"}],"Filters":null,"Actions":[{"Function":"enqueue","Arguments":[{"Function":"123"},{"Function":"456"}]}]}`,
		"cookie=0x0, table=0, duration=9.471s, n_packets=0, n_bytes=0, priority=0 actions=enqueue(123,456)",
	},
	{
		"meta reset_counts",
		" cookie=0x0, duration=0.007s, table=0, n_packets=0, n_bytes=0, idle_timeout=10, reset_counts in_port=2,dl_src=00:44:55:66:77:88 actions=drop",
		`{"Cookie":0,"Table":0,"Priority":0,"Meta":[{"Key":"duration","Value":"0.007s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"idle_timeout","Value":"10"},{"Key":"reset_counts","Value":""}],"Filters":[{"Key":"in_port","Value":"2"},{"Key":"dl_src","Value":"00:44:55:66:77:88"}],"Actions":[{"Function":"drop"}]}`,
		"cookie=0x0, table=0, duration=0.007s, n_packets=0, n_bytes=0, idle_timeout=10, reset_counts, priority=0,in_port=2,dl_src=00:44:55:66:77:88 actions=drop",
	},
	{
		"meta importance",
		" cookie=0x0, duration=0.089s, table=0, n_packets=0, n_bytes=0, hard_timeout=505, importance=35, priority=10,in_port=2 actions=drop",
		`{"Cookie":0,"Table":0,"Priority":10,"Meta":[{"Key":"duration","Value":"0.089s"},{"Key":"n_packets","Value":"0"},{"Key":"n_bytes","Value":"0"},{"Key":"hard_timeout","Value":"505"},{"Key":"importance","Value":"35"}],"Filters":[{"Key":"in_port","Value":"2"}],"Actions":[{"Function":"drop"}]}`,
		"cookie=0x0, table=0, duration=0.089s, n_packets=0, n_bytes=0, hard_timeout=505, importance=35, priority=10,in_port=2 actions=drop",
	},
}

var testGroupParses = []struct {
	name     string
	openflow string
	json     string
	pretty   string
}{
	{
		"group example",
		"group_id=1,type=all,bucket=bucket_id:0,actions=set_field:00:00:00:11:11:11->eth_src,set_field:00:00:00:22:22:22->eth_dst,output:2,bucket=bucket_id:1,actions=set_field:00:00:00:11:11:11->eth_src,set_field:00:00:00:22:22:22->eth_dst,output:v3",
		`{"GroupId":1,"Type":"all","Buckets":[{"Id":0,"Actions":[{"Function":"set_field","Arguments":[{"Function":"00:00:00:11:11:11"},null,{"Function":"eth_src"}]},{"Function":"set_field","Arguments":[{"Function":"00:00:00:22:22:22"},null,{"Function":"eth_dst"}]},{"Function":"output","Arguments":[{"Function":"2"}]}]},{"Id":1,"Actions":[{"Function":"set_field","Arguments":[{"Function":"00:00:00:11:11:11"},null,{"Function":"eth_src"}]},{"Function":"set_field","Arguments":[{"Function":"00:00:00:22:22:22"},null,{"Function":"eth_dst"}]},{"Function":"output","Arguments":[{"Function":"v3"}]}]}]}`,
		"group_id=1, type=all, bucket=bucket_id:0,actions=set_field(00:00:00:11:11:11,,eth_src),set_field(00:00:00:22:22:22,,eth_dst),output(2), bucket=bucket_id:1,actions=set_field(00:00:00:11:11:11,,eth_src),set_field(00:00:00:22:22:22,,eth_dst),output(v3)",
	},
	{
		"Fast FailOver",
		" group_id=2,type=ff,bucket=bucket_id:0,watch_port:1,actions=output:2,bucket=bucket_id:1,watch_port:1,actions=output:v3",
		`{"GroupId":2,"Type":"ff","Buckets":[{"Id":0,"Meta":[{"Key":"watch_port:1","Value":""}],"Actions":[{"Function":"output","Arguments":[{"Function":"2"}]}]},{"Id":1,"Meta":[{"Key":"watch_port:1","Value":""}],"Actions":[{"Function":"output","Arguments":[{"Function":"v3"}]}]}]}`,
		"group_id=2, type=ff, bucket=bucket_id:0,watch_port:1,actions=output(2), bucket=bucket_id:1,watch_port:1,actions=output(v3)",
	},
	{
		"Select",
		" group_id=4,type=select,selection_method=hash,fields(eth_src,eth_dst),bucket=bucket_id:0,actions=output:2,bucket=bucket_id:1,weight:2,actions=output:v3",
		`{"GroupId":4,"Type":"select","Meta":[{"Key":"selection_method","Value":"hash"},{"Key":"fields(eth_src,eth_dst)","Value":""}],"Buckets":[{"Id":0,"Actions":[{"Function":"output","Arguments":[{"Function":"2"}]}]},{"Id":1,"Meta":[{"Key":"weight:2","Value":""}],"Actions":[{"Function":"output","Arguments":[{"Function":"v3"}]}]}]}`,
		"group_id=4, type=select, selection_method=hash, fields(eth_src,eth_dst), bucket=bucket_id:0,actions=output(2), bucket=bucket_id:1,weight:2,actions=output(v3)",
	},
}

func TestScanner(t *testing.T) {
	for _, testCase := range testScans {
		t.Run(testCase.name, func(t *testing.T) {
			stream := NewStream(strings.NewReader(testCase.openflow))
			for i, expectedToken := range testCase.tokens {
				token, _ := stream.scan()
				if token != expectedToken {
					t.Errorf(
						"Found token %s instead of %s at position %d",
						TokenNames[token], TokenNames[expectedToken], i)
				}
				if token == tEOF {
					break
				}
			}
		})
	}
}

func TestParser(t *testing.T) {
	for i, testCase := range testParses {
		name := fmt.Sprintf("TestParser-%d %s", i, testCase.name)
		t.Run(name, func(t *testing.T) {
			ast, err := ToAST(testCase.openflow)
			if err != nil {
				t.Errorf("Failed to generate json")
			} else {
				jsb, _ := json.Marshal(ast)
				js := string(jsb)
				if js != testCase.json {
					t.Errorf(
						"Not the expected output:\n  - %s\n  - %s",
						js, testCase.json)
				}
				pretty := PrettyAST(ast)
				if pretty != testCase.pretty {
					t.Errorf(
						"Not the expected pretty output:\n  - %s\n  - %s",
						pretty, testCase.pretty)
				}
			}
		})
	}
}

func TestGroupParser(t *testing.T) {
	for i, testCase := range testGroupParses {
		name := fmt.Sprintf("TestGroupParser-%d %s", i, testCase.name)
		t.Run(name, func(t *testing.T) {
			ast, err := ToASTGroup(testCase.openflow)
			if err != nil {
				t.Errorf("Failed to generate json: %s", err)
			} else {
				jsb, _ := json.Marshal(ast)
				js := string(jsb)
				if js != testCase.json {
					t.Errorf(
						"Not the expected output:\n  - %s\n  - %s",
						js, testCase.json)
				}
				pretty := PrettyASTGroup(ast)
				if pretty != testCase.pretty {
					t.Errorf(
						"Not the expected pretty output:\n  - %s\n  - %s",
						pretty, testCase.pretty)
				}
			}
		})
	}
}
