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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// OvsOfProbe is the type of the probe retrieving Openflow rules on an Open Vswitch
type OvsOfProbe struct {
	sync.Mutex
	Host         string                    // The host
	Graph        *graph.Graph              // The graph that will receive the rules found
	Root         *graph.Node               // The root node of the host in the graph
	BridgeProbes map[string]*BridgeOfProbe // The table of probes associated to each bridge
	Translation  map[string]string         // A translation table to find the url for a given bridge knowing its name
	Certificate  string                    // Path to the certificate used for authenticated communication with bridges
	PrivateKey   string                    // Path of the private key authenticating the probe.
	CA           string                    // Path of the certicate of the Certificate authority used for authenticated communication with bridges
	sslOk        bool                      // cert private key and ca are provisionned.
	ctx          context.Context
}

// BridgeOfProbe is the type of the probe retrieving Openflow rules on a Bridge.
//
// An important notion is the rawUUID of a rule or the UUID obtained by ignoring the priority from the
// rule filter. Several rules may differ only by their priority (and associated actions). In practice the
// highest priority hides the other rules. It is important to handle rules with the same rawUUID as a group
// because ovs-ofctl monitor does not report priorities.
type BridgeOfProbe struct {
	Host           string               // The global host
	Bridge         string               // The bridge monitored
	UUID           string               // The UUID of the bridge node
	Address        string               // The address of the bridge if different from name
	BridgeNode     *graph.Node          // the bridge node on which the rule nodes are attached.
	OvsOfProbe     *OvsOfProbe          // Back pointer to the probe
	Rules          map[string][]*Rule   // The set of rules found so far grouped by rawUUID
	Groups         map[uint]*graph.Node // The set of groups found so far
	groupMonitored bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// Rule is an OpenFlow rule in a switch
type Rule struct {
	Cookie   uint64 // cookie value of the rule
	Table    int    // table containing the rule
	Priority int    // priority of rule
	Filter   string // all the filters as a comma separated string
	Actions  string // all the actions (comma separated)
	InPort   int    // -1 is any
	UUID     string // Unique id
}

// Group is an OpenFlow group in a switch
type Group struct {
	ID        uint   // Group id
	GroupType string // Group type
	Contents  string // Anything else (buckets + selection)
	UUID      string // Unique id
}

// Event is an event as monitored by ovs-ofctl monitor <br> watch:
type Event struct {
	RawRule *Rule   // The rule from the event
	Rules   []*Rule // Rules found by ovs-ofctl matching the event rule filter.
	Date    int64   // the date of the event
	Action  string  // the action taken
	Bridge  string  // The bridge whtere it ocured
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
func fillIn(components []string, rule *Rule, event *Event) {
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
			case "actions":
				rule.Actions = value
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
}

// extractPriority parses the filter of a rule and extracts the priority if it exists.
func extractPriority(rule *Rule) {
	components := strings.Split(rule.Filter, ",")
	rule.Priority = 32768 // Default rule priority.
	for _, component := range components {
		keyvalue := strings.SplitN(component, "=", 2)
		if len(keyvalue) == 2 {
			key := keyvalue[0]
			value := keyvalue[1]
			switch key {
			case "priority":
				priority, err := strconv.ParseInt(value, 10, 32)
				if err == nil {
					rule.Priority = int(priority)
				} else {
					logging.GetLogger().Errorf("Error while parsing priority of rule: %s", err)
				}
			}
		}
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
	var rule Rule

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
	tentative := len(components) - 1
	if strings.HasPrefix(components[tentative], "actions=") {
		tentative = tentative - 1
	}
	if !strings.HasPrefix(components[tentative], "cookie=") {
		rule.Filter = components[tentative]
	}
	result.RawRule = &rule
	fillUUID(&rule, prefix)
	result.Date = time.Now().Unix()
	result.Bridge = bridge
	return result, nil
}

// Generates a unique UUID for the rule
// prefix is a unique string per bridge using bridge and host names.
func fillUUID(rule *Rule, prefix string) {
	id := prefix + rule.Filter + "-" + string(rule.Table)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		rule.UUID = u.String()
	}
}

// Generate a unique UUID for the group
func fillGroupUUID(group *Group, prefix string) {
	id := prefix + "-" + string(group.ID)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		group.UUID = u.String()
	}
}

// parseRule transforms a single line of ofctl dump-flow in a rule.
// The line DOES NOT include the terminating newline. Protected commas will be replaced.
func parseRule(line string) (*Rule, error) {
	var rule Rule
	if len(line) == 0 || line[0] != ' ' {
		return nil, errors.New("No rule: " + line)
	}
	if strings.ContainsRune(line, '(') {
		line = protectCommas(line)
	}
	components := strings.Split(line[1:], ", ")
	if len(components) < 2 {
		return nil, errors.New("Rule syntax")
	}
	fillIn(components, &rule, nil)
	tail := components[len(components)-1]
	tail = strings.TrimPrefix(tail, "reset_counts ")
	components = strings.Split(tail, " actions=")
	if len(components) == 2 {
		rule.Filter = components[0]
		rule.Actions = components[1]
	} else {
		return nil, errors.New("Rule syntax split filter and actions")
	}
	return &rule, nil
}

// parseGroup transform a single line of ofctl dump-groups in a group
func parseGroup(line string) *Group {
	submatch := groupRegexp.FindStringSubmatch(line)
	if submatch == nil {
		return nil
	}
	groupID, err := strconv.ParseInt(submatch[1], 10, 32)
	if err != nil {
		logging.GetLogger().Warningf("Bad group id in %s", line)
		return nil
	}
	return &Group{
		ID:        uint(groupID),
		GroupType: submatch[2],
		Contents:  submatch[3],
	}
}

func makeFilter(rule *Rule) string {
	if rule.Filter == "" {
		return fmt.Sprintf("table=%d", rule.Table)
	}
	return fmt.Sprintf("table=%d,%s", rule.Table, rule.Filter)
}

// Execute exposes an interface to command launch on the OS
type Execute interface {
	ExecCommand(string, ...string) ([]byte, error)
	ExecCommandPipe(context.Context, string, ...string) (io.Reader, interface {
		Wait() error
	}, error)
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
func (r RealExecute) ExecCommandPipe(ctx context.Context, com string, args ...string) (io.Reader, interface {
	Wait() error
}, error) {
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

func (probe *BridgeOfProbe) prefix() string {
	return probe.OvsOfProbe.Host + "-" + probe.Bridge + "-"
}

func (probe *BridgeOfProbe) dumpGroups() error {
	ofp := probe.OvsOfProbe
	prefix := probe.prefix()
	command, err := ofp.makeCommand(
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
		group := parseGroup(line)
		if group != nil {
			fillGroupUUID(group, prefix)
			probe.addGroup(group)
		}
	}
	return nil
}

func (probe *BridgeOfProbe) getGroup(groupID uint) *Group {
	ofp := probe.OvsOfProbe
	prefix := probe.prefix()
	command, err := ofp.makeCommand(
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
		group := parseGroup(line)
		if group != nil && group.ID == groupID {
			fillGroupUUID(group, prefix)
			return group
		}
	}
	return nil
}

// completeEvent completes the event by looking at it again but with dump-flows and a filter including table. This gives back more elements such as priority.
func completeEvent(ctx context.Context, o *OvsOfProbe, event *Event, prefix string) error {
	oldrule := event.RawRule
	bridge := event.Bridge
	// We want exactly n+1 items where n was the number of items in old filters. The reason is that now
	// the priority is provided. Another approach would be to use the shortest filter as it is the more generic
	expected := countElements(oldrule.Filter)
	filter := makeFilter(oldrule)
	versions := strings.Join(config.GetStringSlice("ovs.oflow.openflow_versions"), ",")
	command, err1 := o.makeCommand(
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
		rule, err2 := parseRule(line)
		if err2 == nil && countElements(rule.Filter) == expected && oldrule.Cookie == rule.Cookie {
			fillUUID(rule, prefix)
			extractPriority(rule)
			event.Rules = append(event.Rules, rule)
		}
	}
	return nil
}

// addRule adds a rule to the graph and links it to the bridge.
func (probe *BridgeOfProbe) addRule(rule *Rule) {
	logging.GetLogger().Infof("New rule %v added", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := probe.BridgeNode
	metadata := graph.Metadata{
		"Type":     "ofrule",
		"cookie":   fmt.Sprintf("0x%x", rule.Cookie),
		"table":    rule.Table,
		"filters":  rule.Filter,
		"actions":  rule.Actions,
		"priority": rule.Priority,
		"UUID":     rule.UUID,
	}
	ruleNode, err := g.NewNode(graph.GenID(), metadata)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	if _, err := g.Link(bridgeNode, ruleNode, graph.Metadata{"RelationType": "ownership"}); err != nil {
		logging.GetLogger().Error(err)
	}
}

// modRule modifies the node of an existing rule.
func (probe *BridgeOfProbe) modRule(rule *Rule) {
	logging.GetLogger().Infof("Rule %v modified", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()

	ruleNode := g.LookupFirstNode(graph.Metadata{"UUID": rule.UUID})
	if ruleNode != nil {
		tr := g.StartMetadataTransaction(ruleNode)
		defer tr.Commit()
		tr.AddMetadata("actions", rule.Actions)
		tr.AddMetadata("cookie", rule.Cookie)
	}
}

// addGroup adds a group to the graph and links it to the bridge
func (probe *BridgeOfProbe) addGroup(group *Group) {
	logging.GetLogger().Infof("New group %v added", group.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := probe.BridgeNode
	metadata := graph.Metadata{
		"Type":       "ofgroup",
		"group_id":   group.ID,
		"group_type": group.GroupType,
		"contents":   group.Contents,
		"UUID":       group.UUID,
	}
	groupNode, err := g.NewNode(graph.GenID(), metadata)
	if err != nil {
		logging.GetLogger().Error(err)
		return
	}
	probe.Groups[group.ID] = groupNode
	if _, err := g.Link(bridgeNode, groupNode, graph.Metadata{"RelationType": "ownership"}); err != nil {
		logging.GetLogger().Error(err)
	}
}

// delRule deletes a rule from the the graph.
func (probe *BridgeOfProbe) delRule(rule *Rule) {
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
func (probe *BridgeOfProbe) delGroup(groupID uint) {
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
func (probe *BridgeOfProbe) modGroup(groupID uint) {
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
	tr.AddMetadata("group_id", group.ID)
	tr.AddMetadata("group_type", group.GroupType)
	tr.AddMetadata("contents", group.Contents)
	tr.AddMetadata("UUID", group.UUID)
}

// containsRule checks if the searched rule may replace an existing rule in
// rules and gives back the coresponding rule if found.
func containsRule(rules []*Rule, searched *Rule) *Rule {
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

// monitor monitors the openflow rules of a bridge by launching a goroutine. The context is used to control the execution of the routine.
func (probe *BridgeOfProbe) monitor(ctx context.Context) error {
	ofp := probe.OvsOfProbe
	command, err1 := ofp.makeCommand([]string{"ovs-ofctl", "monitor"}, probe.Bridge, "watch:")
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
				err = completeEvent(ctx, ofp, &event, prefix)
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
							if found.Actions != rule.Actions || found.Cookie != rule.Cookie {
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

// monitorGroup monitors the openflow groups of a bridge by
// launching a goroutine. It uses OpenFlow 1.4 ForwardRequest
// command that is not directly available in ovs-ofctl
//
// Note: Openflow15 is really needed. Receiving an insert_bucket on an OF14
// monitor crashes the switch (yes, the switch itself) on OVS 2.9
func (probe *BridgeOfProbe) monitorGroup() error {
	if probe.groupMonitored {
		return nil
	}
	// We check that OF1.5 is supported
	ofp := probe.OvsOfProbe
	command, err := ofp.makeCommand(
		[]string{"ovs-ofctl", "-O", "Openflow15", "show"},
		probe.Bridge)
	if err != nil {
		return err
	}
	if _, err = launchOnSwitch(command); err != nil {
		logging.GetLogger().Warningf("OpenFlow 1.5 not supported on %s", probe.Bridge)
		return err
	}
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	controlChannel := filepath.Join(os.TempDir(), "skyof-"+hex.EncodeToString(randBytes))
	command, err = ofp.makeCommand(
		[]string{"ovs-ofctl", "-O", "Openflow15", "monitor", "--unixctl", controlChannel},
		probe.Bridge)
	if err != nil {
		return err
	}
	probe.groupMonitored = true
	lines, err := launchContinuousOnSwitch(probe.ctx, command)
	if err != nil {
		logging.GetLogger().Errorf("Group monitor failed %s: %s", probe.Bridge, err)
		return err
	}
	if err = waitForFile(controlChannel); err != nil {
		logging.GetLogger().Errorf("No control channel for group monitor failed %s: %s", probe.Bridge, err)
		return err
	}
	if err = sendOpenflow(controlChannel, openflowSetSlave); err != nil {
		logging.GetLogger().Errorf("Openflow command 'set slave' for group monitor failed %s: %s", probe.Bridge, err)
		return err
	}
	if err = sendOpenflow(controlChannel, openflowActivateForwardRequest); err != nil {
		logging.GetLogger().Errorf("Openflow command 'activate forward request' for group monitor failed %s: %s", probe.Bridge, err)
		return err
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

// NewBridgeProbe creates a probe and launch the active process
func (o *OvsOfProbe) NewBridgeProbe(host string, bridge string, uuid string, bridgeNode *graph.Node) (*BridgeOfProbe, error) {
	ctx, cancel := context.WithCancel(o.ctx)
	address, ok := o.Translation[bridge]
	if !ok {
		address = bridge
	}
	probe := &BridgeOfProbe{
		Host:           host,
		Bridge:         bridge,
		UUID:           uuid,
		Address:        address,
		BridgeNode:     bridgeNode,
		OvsOfProbe:     o,
		Rules:          make(map[string][]*Rule),
		Groups:         make(map[uint]*graph.Node),
		groupMonitored: false,
		ctx:            ctx,
		cancel:         cancel}

	err := probe.monitor(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	err = probe.monitorGroup()
	if err != nil {
		logging.GetLogger().Errorf("Cannot add group probe on %s - %s", bridge, err)
	}
	return probe, nil
}

func (o *OvsOfProbe) makeCommand(commands []string, bridge string, args ...string) ([]string, error) {
	if strings.HasPrefix(bridge, "ssl:") {
		if o.sslOk {
			commands = append(commands,
				bridge,
				"--certificate", o.Certificate,
				"--ca-cert", o.CA, "--private-key", o.PrivateKey)
		} else {
			return commands, errors.New("Certificate, CA and private keys are necessary for communication with switch over SSL")
		}
	} else {
		commands = append(commands, bridge)
	}
	return append(commands, args...), nil
}

// OnOvsBridgeAdd is called when a bridge is added
func (o *OvsOfProbe) OnOvsBridgeAdd(bridgeNode *graph.Node) {
	o.Lock()
	defer o.Unlock()

	uuid, _ := bridgeNode.GetFieldString("UUID")
	if probe, ok := o.BridgeProbes[uuid]; ok {
		if !probe.groupMonitored {
			probe.monitorGroup()
		}
		return
	}

	hostName := o.Host
	bridgeName, _ := bridgeNode.GetFieldString("Name")
	bridgeProbe, err := o.NewBridgeProbe(hostName, bridgeName, uuid, bridgeNode)
	if err == nil {
		logging.GetLogger().Debugf("Probe added for %s on %s (%s)", bridgeName, hostName, uuid)
		o.BridgeProbes[uuid] = bridgeProbe
	} else {
		logging.GetLogger().Errorf("Cannot add probe for bridge %s on %s (%s): %s", bridgeName, hostName, uuid, err)
	}
}

// OnOvsBridgeDel is called when a bridge is deleted
func (o *OvsOfProbe) OnOvsBridgeDel(uuid string) {
	o.Lock()
	defer o.Unlock()

	if bridgeProbe, ok := o.BridgeProbes[uuid]; ok {
		bridgeProbe.cancel()
		delete(o.BridgeProbes, uuid)
	}
	// Clean all the rules attached to the bridge.
	g := o.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := o.Graph.LookupFirstNode(graph.Metadata{"UUID": uuid})
	if bridgeNode != nil {
		rules := g.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofrule"}, nil)
		for _, ruleNode := range rules {
			logging.GetLogger().Infof("Rule %v deleted (Bridge deleted)", ruleNode.Metadata["UUID"])
			if err := g.DelNode(ruleNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}
		groups := g.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofgroup"}, nil)
		for _, groupNode := range groups {
			logging.GetLogger().Infof("Group %v deleted (Bridge deleted)", groupNode.Metadata["UUID"])
			if err := g.DelNode(groupNode); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	}
}

// NewOvsOfProbe creates a new probe associated to a given graph, root node and host.
func NewOvsOfProbe(ctx context.Context, g *graph.Graph, root *graph.Node, host string) *OvsOfProbe {
	if !config.GetBool("ovs.oflow.enable") {
		return nil
	}

	logging.GetLogger().Infof("Adding OVS probe on %s", host)
	translate := config.GetStringMapString("ovs.oflow.address")
	cert := config.GetString("ovs.oflow.cert")
	pk := config.GetString("ovs.oflow.key")
	ca := config.GetString("ovs.oflow.ca")
	sslOk := (pk != "") && (ca != "") && (cert != "")
	o := &OvsOfProbe{
		Host:         host,
		Graph:        g,
		Root:         root,
		BridgeProbes: make(map[string]*BridgeOfProbe),
		Translation:  translate,
		Certificate:  cert,
		PrivateKey:   pk,
		CA:           ca,
		sslOk:        sslOk,
		ctx:          ctx,
	}
	return o
}
