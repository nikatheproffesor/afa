// Package network observes operating-system routes without sending traffic.
package network

import "time"

type Observation struct {
	Supported      bool      `json:"supported"`
	InterfaceIndex uint32    `json:"interface_index,omitempty"`
	ConnectionType string    `json:"connection_type"`
	ObservedAt     time.Time `json:"observed_at"`
}
