// Code generated - DO NOT EDIT.

package blockdev

import (
	"github.com/skydive-project/skydive/common"
	"strings"
)

func (obj *Metadata) GetFieldBool(key string) (bool, error) {
	switch key {
	case "DiscZero":
		return obj.DiscZero, nil
	case "Hotplug":
		return obj.Hotplug, nil
	case "Rand":
		return obj.Rand, nil
	case "Rm":
		return obj.Rm, nil
	case "Ro":
		return obj.Ro, nil
	case "Rota":
		return obj.Rota, nil
	}

	return false, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Alignment":
		return int64(obj.Alignment), nil
	case "DiscAln":
		return int64(obj.DiscAln), nil
	case "LogSec":
		return int64(obj.LogSec), nil
	case "MinIo":
		return int64(obj.MinIo), nil
	case "OptIo":
		return int64(obj.OptIo), nil
	case "PhySec":
		return int64(obj.PhySec), nil
	case "Ra":
		return int64(obj.Ra), nil
	case "RqSize":
		return int64(obj.RqSize), nil
	}
	return 0, common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldString(key string) (string, error) {
	switch key {
	case "Index":
		return string(obj.Index), nil
	case "Name":
		return string(obj.Name), nil
	case "DiscGran":
		return string(obj.DiscGran), nil
	case "DiscMax":
		return string(obj.DiscMax), nil
	case "Fsavail":
		return string(obj.Fsavail), nil
	case "Fssize":
		return string(obj.Fssize), nil
	case "Fstype":
		return string(obj.Fstype), nil
	case "FsusePercent":
		return string(obj.FsusePercent), nil
	case "Fsused":
		return string(obj.Fsused), nil
	case "Group":
		return string(obj.Group), nil
	case "Hctl":
		return string(obj.Hctl), nil
	case "Kname":
		return string(obj.Kname), nil
	case "Label":
		return string(obj.Label), nil
	case "MajMin":
		return string(obj.MajMin), nil
	case "Mode":
		return string(obj.Mode), nil
	case "Model":
		return string(obj.Model), nil
	case "Mountpoint":
		return string(obj.Mountpoint), nil
	case "Owner":
		return string(obj.Owner), nil
	case "Partflags":
		return string(obj.Partflags), nil
	case "Partlabel":
		return string(obj.Partlabel), nil
	case "Parttype":
		return string(obj.Parttype), nil
	case "Partuuid":
		return string(obj.Partuuid), nil
	case "Path":
		return string(obj.Path), nil
	case "Pkname":
		return string(obj.Pkname), nil
	case "Pttype":
		return string(obj.Pttype), nil
	case "Ptuuid":
		return string(obj.Ptuuid), nil
	case "Rev":
		return string(obj.Rev), nil
	case "Sched":
		return string(obj.Sched), nil
	case "Serial":
		return string(obj.Serial), nil
	case "Size":
		return string(obj.Size), nil
	case "State":
		return string(obj.State), nil
	case "Subsystems":
		return string(obj.Subsystems), nil
	case "Tran":
		return string(obj.Tran), nil
	case "Type":
		return string(obj.Type), nil
	case "UUID":
		return string(obj.UUID), nil
	case "Vendor":
		return string(obj.Vendor), nil
	case "Wsame":
		return string(obj.Wsame), nil
	case "WWN":
		return string(obj.WWN), nil
	}
	return "", common.ErrFieldNotFound
}

func (obj *Metadata) GetFieldKeys() []string {
	return []string{
		"Index",
		"Name",
		"Alignment",
		"DiscAln",
		"DiscGran",
		"DiscMax",
		"DiscZero",
		"Fsavail",
		"Fssize",
		"Fstype",
		"FsusePercent",
		"Fsused",
		"Group",
		"Hctl",
		"Hotplug",
		"Kname",
		"Label",
		"LogSec",
		"MajMin",
		"MinIo",
		"Mode",
		"Model",
		"Mountpoint",
		"OptIo",
		"Owner",
		"Partflags",
		"Partlabel",
		"Parttype",
		"Partuuid",
		"Path",
		"PhySec",
		"Pkname",
		"Pttype",
		"Ptuuid",
		"Ra",
		"Rand",
		"Rev",
		"Rm",
		"Ro",
		"Rota",
		"RqSize",
		"Sched",
		"Serial",
		"Size",
		"State",
		"Subsystems",
		"Tran",
		"Type",
		"UUID",
		"Vendor",
		"Wsame",
		"WWN",
		"Labels",
	}
}

func (obj *Metadata) MatchBool(key string, predicate common.BoolPredicate) bool {
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Metadata) MatchInt64(key string, predicate common.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Metadata) MatchString(key string, predicate common.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Labels":
		if index != -1 && obj.Labels != nil {
			return obj.Labels.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *Metadata) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Labels":
		if obj.Labels != nil {
			if index != -1 {
				return obj.Labels.GetField(key[index+1:])
			} else {
				return obj.Labels, nil
			}
		}

	}
	return nil, common.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
