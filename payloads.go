package clip

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type NodeAnnouncement struct{}

type NodeInfo struct {
	About             *string           `json:"about,omitempty" yaml:"about,omitempty"`
	MaxChannelSizeSat *uint64           `json:"max_channel_size_sat,omitempty" yaml:"max_channel_size_sat,omitempty" validate:"omitempty,gtefield=MinChannelSizeSat"`
	MinChannelSizeSat *uint64           `json:"min_channel_size_sat,omitempty" yaml:"min_channel_size_sat,omitempty"`
	ContactInfo       []ContactInfo     `json:"contact_info,omitempty" yaml:"contact_info,omitempty" validate:"dive"`
	CustomRecords     map[string]string `json:"custom_records,omitempty" yaml:"custom_records,omitempty"`
}

type ContactInfo struct {
	Type    string `json:"type" yaml:"type" validate:"required"`
	Value   string `json:"value" yaml:"value" validate:"required"`
	Note    string `json:"note,omitempty" yaml:"note,omitempty"`
	Primary bool   `json:"primary,omitempty" yaml:"primary,omitempty"`
}

func (n *NodeInfo) Validate() error {
	// Validate all fields with required tag
	if err := validate.Struct(n); err != nil {
		return err
	}

	// ensure at most one ContactInfo has Primary == true
	count := 0
	for _, c := range n.ContactInfo {
		if c.Primary {
			count++
			if count > 1 {
				return errors.New("only one contact info may be primary")
			}
		}
	}
	return nil
}
