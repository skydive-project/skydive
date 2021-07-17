package ovnmodel

// OVNLink describes a Link between two OVN tables
type OVNLink struct {
	SourceTable  string // source table name
	SourceField  string // source field name
	DestTable    string // destination table name
	DestField    string // destination field table name
	Relationship string // relationship type
}
