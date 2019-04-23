/*
 * Copyright (C) 2017 Orange.
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

package ovsdb

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skydive-project/skydive/topology"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/probes/ovsdb/jsonof"
)

// OfctlProbe describes an ovs-ofctl based OpenFlow rule prober.
// An important notion is the rawUUID of a rule or the UUID obtained by ignoring the priority from the
// rule filter. Several rules may differ only by their priority (and associated actions). In practice the
// highest priority hides the other rules. It is important to handle rules with the same rawUUID as a group
// because ovs-ofctl monitor does not report priorities.
type OfctlProbe struct {
	Host           string                        // The global host
	Bridge         string                        // The bridge monitored
	UUID           string                        // The UUID of the bridge node
	Address        string                        // The address of the bridge if different from name
	BridgeNode     *graph.Node                   // the bridge node on which the rule nodes are attached.
	OvsOfProbe     *OvsOfProbe                   // Back pointer to the probe
	Rules          map[string][]*jsonof.JSONRule // The set of rules found so far grouped by rawUUID
	Groups         map[uint]*graph.Node          // The set of groups found so far
	groupMonitored bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// RawRule is an OpenFlow rule in a switch captured as an event.
type RawRule struct {
	Cookie   uint64 // cookie value of the rule
	Table    int    // table containing the rule
	Priority int    // priority of rule
	Filter   string // all the filters as a comma separated string
	UUID     string // Unique id
}

// Event is an event as monitored by ovs-ofctl monitor <br> watch:
type Event struct {
	RawRule *RawRule           // The rule from the event
	Rules   []*jsonof.JSONRule // Rules found by ovs-ofctl matching the event rule filter.
	Date    int64              // the date of the event
	Action  string             // the action taken
	Bridge  string             // The bridge whtere it ocured
}

const (
	// Set the monitor in slave mode
	openflowSetSlave = "061800180000000300000003000000008000000000000002"
	// activate forward to the monitor of group related requests
	// OVS tests use the following value "061c00280000000200000008000000050002000800000002000400080000001a000a000800000001"
	// But the added asynchronous messages should not be needed for our use case
	openflowActivateForwardRequest = "061c001000000002000a000800000001"
)

// A regular expression to parse group events
var groupEventRegexp = regexp.MustCompile("[ ]*(ADD|DEL|MOD|INSERT_BUCKET|REMOVE_BUCKET)[ ].*group_id=([0-9]*)(.*)\n$")
var groupRegexp = regexp.MustCompile("[ ]*group_id=([0-9]*),type=([^,]*),(.*)$")

// ProtectCommas substitute commas with semicolon
// when inside parenthesis
func protectCommas(line string) string {
	work := []rune(line)
	var braces = 0
	for i, word := range work {
		switch word {
		case '(':
			braces++
		case ')':
			braces--
		case ',':
			if braces > 0 {
				work[i] = ';'
			}
		}
	}
	return string(work)
}

// fillIn is a utility function that takes a splitted rule line
// and fills a Rule/Event structure with it
func fillIn(components []string, rule *RawRule, event *Event) {
	for _, component := range components {
		keyvalue := strings.SplitN(component, "=", 2)
		if len(keyvalue) == 2 {
			key := keyvalue[0]
			value := keyvalue[1]
			switch key {
			case "event":
				if event != nil {
					event.Action = value
				}
			case "table":
				table, err := strconv.ParseInt(value, 10, 32)
				if err == nil {
					rule.Table = int(table)
				} else {
					logging.GetLogger().Errorf("Error while parsing table of rule: %s", err)
				}
			case "cookie":
				v, err := strconv.ParseUint(value, 0, 64)
				if err == nil {
					rule.Cookie = v
				} else {
					logging.GetLogger().Errorf("Error while parsing cookie of rule: %s", err)
				}
			}
		}
	}
	if len(components) < 2 {
		logging.GetLogger().Errorf("Rule syntax for filters")
	}
	tentativeFilterPos := len(components) - 1
	if strings.HasPrefix(components[tentativeFilterPos], "actions=") {
		tentativeFilterPos = tentativeFilterPos - 1
	}
	if !strings.HasPrefix(components[tentativeFilterPos], "cookie=") {
		rule.Filter = components[tentativeFilterPos]
	}
}

type noEventError struct{}

func (e *noEventError) Error() string {
	return "No Event"
}

// parseEvent transforms a single line of ofctl monitor :watch
// in an event. The string must be terminated by a "\n"
// Protected commas will be replaced.
func parseEvent(line string, bridge string, prefix string) (Event, error) {
	var result Event
	var rule RawRule

	if line[0] != ' ' {
		return result, &noEventError{}
	}
	if strings.ContainsRune(line, '(') {
		line = protectCommas(line)
	}
	components := strings.Split(line[1:len(line)-1], " ")
	fillIn(components, &rule, &result)
	if len(components) < 2 {
		return result, errors.New("Rule syntax")
	}

	result.RawRule = &rule
	fillRawUUID(&rule, prefix)
	result.Date = time.Now().Unix()
	result.Bridge = bridge
	return result, nil
}

// fillRawUUID Generates a unique UUID for the rule
// prefix is a unique string per bridge using bridge and host names.
func fillRawUUID(rule *RawRule, prefix string) {
	id := prefix + rule.Filter + "-" + string(rule.Table)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		rule.UUID = u.String()
	}
}

// fillRawUUID Generates a unique UUID for the rule
// prefix is a unique string per bridge using bridge and host names.
func fillUUID(rule *jsonof.JSONRule, prefix string) {
	id := prefix + rule.RawFilter + "-" + string(rule.Table) + "-" + string(rule.Priority)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		rule.UUID = u.String()
	}
}

// Generate a unique UUID for the group
func fillGroupUUID(group *jsonof.JSONGroup, prefix string) {
	id := prefix + "-" + string(group.GroupID)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		group.UUID = u.String()
	}
}

func makeFilter(rule *RawRule) string {
	if rule.Filter == "" {
		return fmt.Sprintf("table=%d", rule.Table)
	}
	return fmt.Sprintf("table=%d,%s", rule.Table, rule.Filter)
}

// Waiter exposes an interface with a simple Wait method
type Waiter interface {
	Wait() error
}

// Execute exposes an interface to command launch on the OS
type Execute interface {
	ExecCommand(string, ...string) ([]byte, error)
	ExecCommandPipe(context.Context, string, ...string) (io.Reader, Waiter, error)
}

// RealExecute is the actual implementation given below. It can be overridden for tests.
type RealExecute struct{}

var executor Execute = RealExecute{}

// ExecCommand executes a command on a host
func (r RealExecute) ExecCommand(com string, args ...string) ([]byte, error) {
	/* #nosec */
	command := exec.Command(com, args...)
	return command.CombinedOutput()
}

// ExecCommandPipe executes a command on a host and gives back a pipe to control it.
func (r RealExecute) ExecCommandPipe(ctx context.Context, com string, args ...string) (io.Reader, Waiter, error) {
	/* #nosec */
	command := exec.CommandContext(ctx, com, args...)
	out, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	command.Stderr = command.Stdout
	if err = command.Start(); err != nil {
		return nil, nil, err
	}

	return out, command, err
}

// launchOnSwitch launches a command on a given switch
func launchOnSwitch(cmd []string) (string, error) {
	bytes, err := executor.ExecCommand(cmd[0], cmd[1:]...)
	if err == nil {
		return string(bytes), nil
	}
	logging.GetLogger().Debugf("Command %v failed: %s", cmd, string(bytes))
	return "", err
}

// launchContinuousOnSwitch launches  a stream producing command on a given switch. The command is resilient
// and is relaunched until the context explicitly cancels it.
func launchContinuousOnSwitch(ctx context.Context, cmd []string) (<-chan string, error) {
	var cout = make(chan string, 10)

	go func() {
		logging.GetLogger().Debugf("Launching continusously %v", cmd)
		for ctx.Err() == nil {
			retry := func() error {
				out, com, err := executor.ExecCommandPipe(ctx, cmd[0], cmd[1:]...)
				if err != nil {
					logging.GetLogger().Errorf("Can't execute command %v: %s", cmd, err)
					return nil
				}

				reader := bufio.NewReader(out)
				for ctx.Err() == nil {
					line, err := reader.ReadString('\n')
					if err == io.EOF {
						com.Wait()
						break
					} else if err != nil {
						logging.GetLogger().Errorf("IO Error on command %v: %s", cmd, err)
						// Should return but there may be weird cases.
						go com.Wait()
						break
					} else {
						if strings.Contains(line, "is not a bridge or a socket") {
							reader.Discard(int(^uint(0) >> 1))
							com.Wait()
							return errors.New("Not a bridge or a socket")
						}
						cout <- line
					}
				}

				return nil
			}
			if err := common.Retry(retry, 100, 50*time.Millisecond); err != nil {
				logging.GetLogger().Error(err)
				break
			}
		}
		close(cout)
	}()

	return cout, nil
}

func countElements(filter string) int {
	if len(filter) == 0 {
		return 1
	}
	elts := strings.Split(filter, ",")
	l := len(elts) + 1
	for _, elt := range elts {
		if strings.HasPrefix(elt, "priority=") {
			l = l - 1
			break
		}
	}
	return l
}

func (probe *OfctlProbe) prefix() string {
	return probe.OvsOfProbe.Host + "-" + probe.Bridge + "-"
}

func (probe *OfctlProbe) dumpGroups() error {
	prefix := probe.prefix()
	command, err := probe.makeCommand(
		[]string{"ovs-ofctl", "-O", "OpenFlow15", "dump-groups"},
		probe.Bridge)
	if err != nil {
		return err
	}
	lines, err := launchOnSwitch(command)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(lines, "\n") {
		group, err := jsonof.ToASTGroup(line)
		if err == nil {
			fillGroupUUID(group, prefix)
			probe.addGroup(group)
		}
	}
	return nil
}

func (probe *OfctlProbe) getGroup(groupID uint) *jsonof.JSONGroup {
	prefix := probe.prefix()
	command, err := probe.makeCommand(
		[]string{"ovs-ofctl", "-O", "OpenFlow15", "dump-groups"},
		probe.Bridge, string(groupID))
	if err != nil {
		return nil
	}
	lines, err := launchOnSwitch(command)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(lines, "\n") {
		group, err := jsonof.ToASTGroup(line)
		if err == nil && group.GroupID == groupID {
			fillGroupUUID(group, prefix)
			return group
		}
	}
	return nil
}

// completeEvent completes the event by looking at it again but with dump-flows and a filter including table. This gives back more elements such as priority.
func (probe *OfctlProbe) completeEvent(ctx context.Context, o *OvsOfProbe, event *Event, prefix string) error {
	oldrule := event.RawRule
	bridge := event.Bridge
	// We want exactly n+1 items where n was the number of items in old filters. The reason is that now
	// the priority is provided. Another approach would be to use the shortest filter as it is the more generic
	expected := countElements(oldrule.Filter)
	filter := makeFilter(oldrule)
	versions := strings.Join(config.GetStringSlice("ovs.oflow.openflow_versions"), ",")
	command, err1 := probe.makeCommand(
		[]string{"ovs-ofctl", "-O", versions, "dump-flows"},
		bridge, filter)
	if err1 != nil {
		return err1
	}
	lines, err := launchOnSwitch(command)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("Cannot launch ovs-ofctl dump-flows on %s@%s with filter %s: %s", bridge, o.Host, filter, err)
	}
	for _, line := range strings.Split(lines, "\n") {
		rule, err2 := jsonof.ToAST(line)
		if err2 == nil && countElements(rule.RawFilter) == expected && oldrule.Cookie == rule.Cookie {
			fillUUID(rule, prefix)
			event.Rules = append(event.Rules, rule)
		}
	}
	return nil
}

// addRule adds a rule to the graph and links it to the bridge.
func (probe *OfctlProbe) addRule(rule *jsonof.JSONRule) {
	logging.GetLogger().Infof("New rule %v added", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := probe.BridgeNode
	metadata := graph.Metadata{
		"Type":     "ofrule",
		"Cookie":   fmt.Sprintf("0x%x", rule.Cookie),
		"Table":    rule.Table,
		"Filters":  rule.Filters,
		"Actions":  rule.Actions,
		"Priority": rule.Priority,
		"UUID":     rule.UUID,
	}
	ruleNode, err := g.NewNode(graph.GenID(), metadata)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	if _, err := topology.AddOwnershipLink(g, bridgeNode, ruleNode, nil); err != nil {
		logging.GetLogger().Error(err)
	}
}

// modRule modifies the node of an existing rule.
func (probe *OfctlProbe) modRule(rule *jsonof.JSONRule) {
	logging.GetLogger().Infof("Rule %v modified", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()

	ruleNode := g.LookupFirstNode(graph.Metadata{"UUID": rule.UUID})
	if ruleNode != nil {
		tr := g.StartMetadataTransaction(ruleNode)
		defer tr.Commit()
		tr.AddMetadata("Actions", rule.Actions)
		tr.AddMetadata("Cookie", rule.Cookie)
	}
}

// addGroup adds a group to the graph and links it to the bridge
func (probe *OfctlProbe) addGroup(group *jsonof.JSONGroup) {
	logging.GetLogger().Infof("New group %v added", group.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := probe.BridgeNode
	metadata := graph.Metadata{
		"Type":      "ofgroup",
		"GroupId":   group.GroupID,
		"GroupType": group.Type,
		"Meta":      group.Meta,
		"Buckets":   group.Buckets,
		"UUID":      group.UUID,
	}
	groupNode, err := g.NewNode(graph.GenID(), metadata)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	probe.Groups[group.GroupID] = groupNode
	if _, err := topology.AddOwnershipLink(g, bridgeNode, groupNode, nil); err != nil {
		logging.GetLogger().Error(err)
	}
}

// delRule deletes a rule from the the graph.
func (probe *OfctlProbe) delRule(rule *jsonof.JSONRule) {
	logging.GetLogger().Infof("Rule %v deleted", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()

	ruleNode := g.LookupFirstNode(graph.Metadata{"UUID": rule.UUID})
	if ruleNode != nil {
		if err := g.DelNode(ruleNode); err != nil {
			logging.GetLogger().Error(err)
		}
	}
}

// delGroup deletes a rule from the the graph.
func (probe *OfctlProbe) delGroup(groupID uint) {
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()

	if groupID == 0xfffffffc {
		logging.GetLogger().Infof("All groups deleted on %s", probe.Bridge)
		for _, groupNode := range probe.Groups {
			if err := g.DelNode(groupNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}
		probe.Groups = make(map[uint]*graph.Node)
	} else {
		logging.GetLogger().Infof("Group %d deleted on %s", groupID, probe.Bridge)
		groupNode := probe.Groups[groupID]
		delete(probe.Groups, groupID)
		if groupNode != nil {
			if err := g.DelNode(groupNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	}
}

// modGroup modifies a group from the graph
func (probe *OfctlProbe) modGroup(groupID uint) {
	logging.GetLogger().Infof("Group %d modified", groupID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	group := probe.getGroup(groupID)
	node := probe.Groups[groupID]
	if group == nil || node == nil {
		logging.GetLogger().Errorf("Cannot modify node of OF group %d", groupID)
		return
	}
	tr := g.StartMetadataTransaction(node)
	defer tr.Commit()
	tr.AddMetadata("GroupId", group.GroupID)
	tr.AddMetadata("GroupType", group.Type)
	tr.AddMetadata("Meta", group.Meta)
	tr.AddMetadata("Buckets", group.Buckets)
	tr.AddMetadata("UUID", group.UUID)
}

// containsRule checks if the searched rule may replace an existing rule in
// rules and gives back the coresponding rule if found.
func containsRule(rules []*jsonof.JSONRule, searched *jsonof.JSONRule) *jsonof.JSONRule {
	for _, rule := range rules {
		if rule.UUID == searched.UUID {
			return rule
		}
	}
	return nil
}

// sendOpenflow sends an Openflow command in hexadecimal through the control
// channel of an existing ovs-ofctl monitor command.
func sendOpenflow(control string, hex string) error {
	command := []string{"ovs-appctl", "-t", control, "ofctl/send", hex}
	_, err := launchOnSwitch(command)
	return err
}

func (probe *OfctlProbe) makeCommand(commands []string, bridge string, args ...string) ([]string, error) {
	if strings.HasPrefix(bridge, "ssl:") {
		if probe.OvsOfProbe.sslOk {
			commands = append(commands,
				bridge,
				"--certificate", probe.OvsOfProbe.Certificate,
				"--ca-cert", probe.OvsOfProbe.CA,
				"--private-key", probe.OvsOfProbe.PrivateKey)
		} else {
			return commands, errors.New("Certificate, CA and private keys are necessary for communication with switch over SSL")
		}
	} else {
		commands = append(commands, bridge)
	}
	return append(commands, args...), nil
}

// Monitor monitors the openflow rules of a bridge by launching a goroutine. The context is used to control the execution of the routine.
func (probe *OfctlProbe) Monitor(ctx context.Context) error {
	probe.ctx = ctx
	ofp := probe.OvsOfProbe
	command, err1 := probe.makeCommand([]string{"ovs-ofctl", "monitor"}, probe.Bridge, "watch:")
	if err1 != nil {
		return err1
	}
	lines, err := launchContinuousOnSwitch(ctx, command)
	if err != nil {
		return err
	}
	go func() {
		logging.GetLogger().Debugf("Launching goroutine for monitor %s", probe.Bridge)
		prefix := probe.Host + "-" + probe.Bridge + "-"
		for line := range lines {
			if ctx.Err() != nil {
				break
			}
			event, err := parseEvent(line, probe.Bridge, prefix)
			if err == nil {
				err = probe.completeEvent(ctx, ofp, &event, prefix)
				if err != nil {
					logging.GetLogger().Errorf("Error while monitoring %s@%s: %s", probe.Bridge, probe.Host, err)
					continue
				}
				rawUUID := event.RawRule.UUID
				oldRules := probe.Rules[rawUUID]
				switch event.Action {
				case "ADDED", "MODIFIED":
					for _, rule := range event.Rules {
						found := containsRule(oldRules, rule)
						if found == nil {
							oldRules = append(oldRules, rule)
							probe.addRule(rule)
						} else {
							if !reflect.DeepEqual(found.Actions, rule.Actions) || found.Cookie != rule.Cookie {
								found.Actions = rule.Actions
								found.Cookie = rule.Cookie
								probe.modRule(rule)
							}
						}
					}
					probe.Rules[rawUUID] = oldRules
				case "DELETED":
					for _, oldRule := range oldRules {
						if containsRule(event.Rules, oldRule) == nil {
							probe.delRule(oldRule)
						}
					}
					if len(event.Rules) == 0 {
						delete(probe.Rules, rawUUID)
					} else {
						probe.Rules[rawUUID] = event.Rules
					}
				}
			} else {
				if _, ok := err.(*noEventError); !ok {
					logging.GetLogger().Errorf("Error while monitoring %s@%s: %s", probe.Bridge, probe.Host, err)
				}
			}

		}
	}()
	return nil
}

// Check if a file is created in time
func waitForFile(path string) error {
	retry := func() error {
		_, err := os.Stat(path)
		return err
	}
	return common.Retry(retry, 10, 500*time.Millisecond)
}

// MonitorGroup monitors the openflow groups of a bridge by
// launching a goroutine. It uses OpenFlow 1.4 ForwardRequest
// command that is not directly available in ovs-ofctl
//
// Note: Openflow15 is really needed. Receiving an insert_bucket on an OF14
// monitor crashes the switch (yes, the switch itself) on OVS 2.9
func (probe *OfctlProbe) MonitorGroup() error {
	if probe.groupMonitored {
		return nil
	}
	// We check that OF1.5 is supported
	command, err := probe.makeCommand(
		[]string{"ovs-ofctl", "-O", "Openflow15", "show"},
		probe.Bridge)
	if err != nil {
		return err
	}
	if _, err = launchOnSwitch(command); err != nil {
		return ErrGroupNotSupported
	}
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	controlChannel := filepath.Join(os.TempDir(), "skyof-"+hex.EncodeToString(randBytes))
	command, err = probe.makeCommand(
		[]string{"ovs-ofctl", "-O", "Openflow15", "monitor", "--unixctl", controlChannel},
		probe.Bridge)
	if err != nil {
		return err
	}
	probe.groupMonitored = true
	lines, err := launchContinuousOnSwitch(probe.ctx, command)
	if err != nil {
		return fmt.Errorf("Group monitor failed %s: %s", probe.Bridge, err)
	}
	if err = waitForFile(controlChannel); err != nil {
		return fmt.Errorf("No control channel for group monitor failed %s: %s", probe.Bridge, err)
	}
	if err = sendOpenflow(controlChannel, openflowSetSlave); err != nil {
		return fmt.Errorf("Openflow command 'set slave' for group monitor failed %s: %s", probe.Bridge, err)
	}
	if err = sendOpenflow(controlChannel, openflowActivateForwardRequest); err != nil {
		return fmt.Errorf("Openflow command 'activate forward request' for group monitor failed %s: %s", probe.Bridge, err)
	}
	go func() {
		logging.GetLogger().Debugf("Launching goroutine for monitor groups %s", probe.Bridge)
		err := probe.dumpGroups()
		if err != nil {
			logging.GetLogger().Warningf("Cannot dump groups for %s: %s", probe.Bridge, err)
			return
		}
		logging.GetLogger().Debugf("Treating events for groups on %s", probe.Bridge)
		for line := range lines {
			if probe.ctx.Err() != nil {
				break
			}
			submatch := groupEventRegexp.FindStringSubmatch(line)
			if submatch == nil {
				continue
			}
			gid, err := strconv.ParseUint(submatch[2], 10, 32)
			if err != nil {
				logging.GetLogger().Errorf("Cannot parse group id %s on %s: %s", submatch[2], probe.Bridge, err)
				continue
			}
			groupID := uint(gid)
			switch submatch[1] {
			case "ADD":
				group := probe.getGroup(groupID)
				if group != nil {
					probe.addGroup(group)
				}
			case "DEL":
				probe.delGroup(groupID)
			case "MOD", "INSERT_BUCKET", "REMOVE_BUCKET":
				probe.modGroup(groupID)
			default:
				logging.GetLogger().Errorf("unknown group action %s", submatch[1])
			}
		}
	}()
	return nil
}

// NewOfctlProbe returns a new ovs-ofctl based OpenFlow probe
func NewOfctlProbe(host, bridge, uuid, address string, bridgeNode *graph.Node, o *OvsOfProbe) *OfctlProbe {
	return &OfctlProbe{
		Host:           host,
		Bridge:         bridge,
		UUID:           uuid,
		Address:        address,
		BridgeNode:     bridgeNode,
		OvsOfProbe:     o,
		Rules:          make(map[string][]*jsonof.JSONRule),
		Groups:         make(map[uint]*graph.Node),
		groupMonitored: false,
	}
}
