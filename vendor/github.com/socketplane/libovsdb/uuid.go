package libovsdb

import (
	"encoding/json"
	"errors"
	"regexp"
)

// UUID is a UUID according to RFC7047
type UUID struct {
	GoUUID string `json:"uuid"`
}

// MarshalJSON will marshal an OVSDB style UUID to a JSON encoded byte array
func (u UUID) MarshalJSON() ([]byte, error) {
	var uuidSlice []string
	err := u.validateUUID()
	if err == nil {
		uuidSlice = []string{"uuid", u.GoUUID}
	} else {
		uuidSlice = []string{"named-uuid", u.GoUUID}
	}

	return json.Marshal(uuidSlice)
}

// UnmarshalJSON will unmarshal a JSON encoded byte array to a OVSDB style UUID
func (u *UUID) UnmarshalJSON(b []byte) (err error) {
	var ovsUUID []string
	if err := json.Unmarshal(b, &ovsUUID); err == nil {
		u.GoUUID = ovsUUID[1]
	}
	return err
}

func (u UUID) validateUUID() error {
	if len(u.GoUUID) != 36 {
		return errors.New("uuid exceeds 36 characters")
	}

	var validUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	if !validUUID.MatchString(u.GoUUID) {
		return errors.New("uuid does not match regexp")
	}

	return nil
}
